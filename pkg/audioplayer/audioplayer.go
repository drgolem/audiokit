package audioplayer

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/drgolem/audiokit/pkg/audioframe"
	"github.com/drgolem/audiokit/pkg/audioframeringbuffer"
	"github.com/drgolem/audiokit/pkg/decoder"
	"github.com/drgolem/audiokit/pkg/types"

	"github.com/drgolem/go-portaudio/portaudio"
)

// AudioPlayer plays audio from any decoder.AudioDecoder using PortAudio callback mode
// with an AudioFrameRingBuffer. Implements SPSC (Single-Producer Single-Consumer) pattern.
//
// Thread Safety Model:
//   - Producer goroutine writes to ringbuffer
//   - PortAudio C thread (audio callback) reads from ringbuffer
//   - Atomic operations for all shared state
//   - Deep copy for frame data to prevent buffer corruption
type AudioPlayer struct {
	ringbuf         *audioframeringbuffer.AudioFrameRingBuffer
	stream          *portaudio.PaStream
	decoder         decoder.AudioDecoder
	deviceIndex     int
	framesPerBuffer int
	samplesPerFrame int

	// Current audio format
	sampleRate     int
	channels       int
	bitsPerSample  int
	bytesPerSample int

	// Goroutine coordination
	producerDone         atomic.Bool
	playbackComplete     atomic.Bool
	playbackCompleteChan chan struct{}
	stopChan             chan struct{}
	wg                   sync.WaitGroup
	mu                   sync.Mutex
	stopped              bool
	draining             atomic.Bool // signals callback to fade out

	// Callback state for partial frame consumption
	currentFrame atomic.Pointer[audioframe.AudioFrame]
	frameOffset  int

	// Playback status tracking
	label           string
	startTime       time.Time
	producedSamples atomic.Uint64
	playedSamples   atomic.Uint64
}

// New creates a new AudioPlayer with the specified configuration.
//
// Parameters:
//   - deviceIdx: PortAudio device index for audio output
//   - bufferCapacity: Ringbuffer capacity in number of AudioFrames
//   - framesPerBuffer: PortAudio frames per buffer callback
//   - samplesPerFrame: Number of samples per AudioFrame
func New(deviceIdx int, bufferCapacity uint64, framesPerBuffer, samplesPerFrame int) *AudioPlayer {
	return &AudioPlayer{
		ringbuf:         audioframeringbuffer.New(bufferCapacity),
		deviceIndex:     deviceIdx,
		framesPerBuffer: framesPerBuffer,
		samplesPerFrame: samplesPerFrame,
	}
}

// SetDecoder sets the audio decoder to play from.
// Closes any previously set decoder.
func (ap *AudioPlayer) SetDecoder(dec decoder.AudioDecoder, label string) {
	if ap.decoder != nil {
		ap.decoder.Close()
		ap.decoder = nil
	}

	rate, channels, bps := dec.GetFormat()

	slog.Info("Audio decoder set",
		"label", label,
		"sample_rate", rate,
		"channels", channels,
		"bits_per_sample", bps)

	ap.decoder = dec
	ap.sampleRate = rate
	ap.channels = channels
	ap.bitsPerSample = bps
	ap.bytesPerSample = bps / 8
	ap.label = label
}

// Play starts playback of the current decoder.
// Returns an error if no decoder is set or if the stream cannot be initialized.
//
// Use Wait() to block until playback completes, or Stop() to interrupt.
func (ap *AudioPlayer) Play() error {
	if ap.decoder == nil {
		return fmt.Errorf("no decoder set")
	}

	// Reset state
	ap.producerDone.Store(false)
	ap.playbackComplete.Store(false)
	ap.draining.Store(false)
	ap.playbackCompleteChan = make(chan struct{})
	ap.stopChan = make(chan struct{})
	ap.stopped = false
	ap.currentFrame.Store(nil)
	ap.frameOffset = 0
	ap.ringbuf.Reset()
	ap.producedSamples.Store(0)
	ap.playedSamples.Store(0)
	ap.startTime = time.Now()

	if err := ap.initializeStream(); err != nil {
		return err
	}

	ap.wg.Add(1)
	go ap.producer()

	slog.Debug("Playback started")
	return nil
}

func (ap *AudioPlayer) initializeStream() error {
	var sampleFormat portaudio.PaSampleFormat
	switch ap.bitsPerSample {
	case 16:
		sampleFormat = portaudio.SampleFmtInt16
	case 24:
		sampleFormat = portaudio.SampleFmtInt24
	case 32:
		sampleFormat = portaudio.SampleFmtInt32
	default:
		return fmt.Errorf("unsupported bit depth: %d", ap.bitsPerSample)
	}

	ap.stream = &portaudio.PaStream{
		OutputParameters: &portaudio.PaStreamParameters{
			DeviceIndex:  ap.deviceIndex,
			ChannelCount: ap.channels,
			SampleFormat: sampleFormat,
		},
		SampleRate: float64(ap.sampleRate),
	}

	if err := ap.stream.OpenCallback(ap.framesPerBuffer, ap.audioCallback); err != nil {
		return fmt.Errorf("failed to open stream with callback: %w", err)
	}

	if err := ap.stream.StartStream(); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}

	return nil
}

// audioCallback is called by PortAudio from a real-time audio thread (not a Go goroutine).
// It is the consumer in the SPSC pattern, reading frames from the ringbuffer.
func (ap *AudioPlayer) audioCallback(
	input, output []byte,
	frameCount uint,
	timeInfo *portaudio.StreamCallbackTimeInfo,
	statusFlags portaudio.StreamCallbackFlags,
) portaudio.StreamCallbackResult {
	bytesNeeded := int(frameCount) * ap.channels * ap.bytesPerSample
	bytesWritten := 0

	if ap.producerDone.Load() && ap.ringbuf.AvailableRead() == 0 && ap.currentFrame.Load() == nil {
		ap.playbackComplete.Store(true)
		select {
		case <-ap.playbackCompleteChan:
		default:
			close(ap.playbackCompleteChan)
		}
		return portaudio.Complete
	}

	for bytesWritten < bytesNeeded {
		currentFrame := ap.currentFrame.Load()
		if currentFrame == nil {
			if ap.ringbuf.AvailableRead() > 0 {
				frames, err := ap.ringbuf.Read(1)
				if err != nil || len(frames) == 0 {
					break
				}
				ap.currentFrame.Store(&frames[0])
				currentFrame = &frames[0]
				ap.frameOffset = 0
			} else {
				break
			}
		}

		remainingInFrame := len(currentFrame.Audio) - ap.frameOffset
		remainingInOutput := bytesNeeded - bytesWritten
		bytesToCopy := min(remainingInFrame, remainingInOutput)

		copy(output[bytesWritten:bytesWritten+bytesToCopy],
			currentFrame.Audio[ap.frameOffset:ap.frameOffset+bytesToCopy])

		bytesWritten += bytesToCopy
		ap.frameOffset += bytesToCopy

		if ap.frameOffset >= len(currentFrame.Audio) {
			ap.currentFrame.Store(nil)
			ap.frameOffset = 0
		}
	}

	if bytesWritten < bytesNeeded {
		clear(output[bytesWritten:bytesNeeded])
	}

	// Fade out when draining (Ctrl+C) to avoid click at stop
	if ap.draining.Load() && bytesWritten > 0 {
		frameBytes := ap.channels * ap.bytesPerSample
		totalFrames := bytesWritten / frameBytes
		for i := 0; i < totalFrames; i++ {
			// Linear fade from 1.0 to 0.0
			gain := 1.0 - float64(i)/float64(totalFrames)
			offset := i * frameBytes
			for ch := 0; ch < ap.channels; ch++ {
				sampleOffset := offset + ch*ap.bytesPerSample
				if ap.bytesPerSample == 2 && sampleOffset+1 < bytesWritten {
					s := int16(output[sampleOffset]) | int16(output[sampleOffset+1])<<8
					s = int16(float64(s) * gain)
					output[sampleOffset] = byte(s)
					output[sampleOffset+1] = byte(s >> 8)
				}
			}
		}
		ap.playbackComplete.Store(true)
		select {
		case <-ap.playbackCompleteChan:
		default:
			close(ap.playbackCompleteChan)
		}
		return portaudio.Complete
	}

	samplesPlayed := bytesWritten / (ap.channels * ap.bytesPerSample)
	ap.playedSamples.Add(uint64(samplesPlayed))

	return portaudio.Continue
}

// producer reads from decoder and writes AudioFrames to ringbuffer.
func (ap *AudioPlayer) producer() {
	defer ap.wg.Done()
	defer ap.producerDone.Store(true)

	bufferBytes := ap.samplesPerFrame * ap.channels * ap.bytesPerSample
	buffer := make([]byte, bufferBytes)

	totalFramesProduced := 0

	for {
		select {
		case <-ap.stopChan:
			slog.Debug("Producer stopped", "total_frames", totalFramesProduced)
			return
		default:
		}

		samplesRead, err := ap.decoder.DecodeSamples(ap.samplesPerFrame, buffer)
		if err != nil || samplesRead == 0 {
			slog.Debug("Producer finished",
				"error", err,
				"samples_read", samplesRead,
				"total_frames", totalFramesProduced)
			return
		}

		bytesToWrite := samplesRead * ap.channels * ap.bytesPerSample

		frame := audioframe.AudioFrame{
			Format: audioframe.FrameFormat{
				SampleRate:    uint32(ap.sampleRate),
				Channels:      uint8(ap.channels),
				BitsPerSample: uint8(ap.bitsPerSample),
			},
			SamplesCount: uint16(samplesRead),
			Audio:        make([]byte, bytesToWrite),
		}
		copy(frame.Audio, buffer[:bytesToWrite])

		toWrite := []audioframe.AudioFrame{frame}
		for len(toWrite) > 0 {
			written, _ := ap.ringbuf.Write(toWrite)
			if written > 0 {
				totalFramesProduced += written
				toWrite = toWrite[written:]
				ap.producedSamples.Add(uint64(samplesRead))
			} else {
				// Yield CPU when ringbuffer is full, avoiding busy-spin
				// that starves the PortAudio callback thread
				time.Sleep(500 * time.Microsecond)
			}

			select {
			case <-ap.stopChan:
				return
			default:
			}
		}
	}
}

// Wait blocks until the current playback finishes.
func (ap *AudioPlayer) Wait() {
	ap.wg.Wait()
	<-ap.playbackCompleteChan
}

// Stop stops playback. Safe to call multiple times.
func (ap *AudioPlayer) Stop() error {
	ap.mu.Lock()
	if ap.stopped {
		ap.mu.Unlock()
		return nil
	}
	ap.stopped = true
	ap.mu.Unlock()

	// Signal callback to fade out before we tear down
	ap.draining.Store(true)
	close(ap.stopChan)
	ap.wg.Wait()

	if ap.stream != nil {
		// Let the callback process one more buffer with fade-out
		time.Sleep(50 * time.Millisecond)
		if err := ap.stream.StopStream(); err != nil {
			slog.Warn("Failed to stop stream", "error", err)
		}
		if err := ap.stream.CloseCallback(); err != nil {
			slog.Warn("Failed to close stream", "error", err)
		}
		ap.stream = nil
	}

	if ap.decoder != nil {
		if err := ap.decoder.Close(); err != nil {
			slog.Warn("Failed to close decoder", "error", err)
		}
		ap.decoder = nil
	}

	return nil
}

// GetPlaybackStatus returns current playback status.
// Implements types.PlaybackMonitor.
func (ap *AudioPlayer) GetPlaybackStatus() types.PlaybackStatus {
	produced := ap.producedSamples.Load()
	played := ap.playedSamples.Load()
	buffered := uint64(0)
	if produced > played {
		buffered = produced - played
	}

	return types.PlaybackStatus{
		FileName:        ap.label,
		SampleRate:      ap.sampleRate,
		Channels:        ap.channels,
		BitsPerSample:   ap.bitsPerSample,
		FramesPerBuffer: ap.framesPerBuffer,
		PlayedSamples:   played,
		BufferedSamples: buffered,
		ElapsedTime:     time.Since(ap.startTime),
	}
}
