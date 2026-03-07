package tempo

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Muges/go-tsm/multichannel"
	"github.com/Muges/go-tsm/tsm"
	"github.com/Muges/go-tsm/wsola"
)

// Processor wraps go-tsm WSOLA for real-time playback speed adjustment.
// Internally always processes mono audio to avoid stereo panning artifacts
// from independent per-channel cross-correlation. Stereo input is downmixed
// to mono before WSOLA, then duplicated back to stereo on output.
// Thread-safe: SetSpeed() can be called from any goroutine.
type Processor struct {
	tsm   *tsm.TSM
	speed float64
	mu    sync.Mutex // protects tsm operations (Put/Receive are not thread-safe)
}

// NewProcessor creates a WSOLA tempo processor for the given speed.
// Internally uses mono processing regardless of input channel count.
func NewProcessor(speed float64) (*Processor, error) {
	if speed < 0.5 || speed > 3.0 {
		return nil, fmt.Errorf("speed out of range (0.5-3.0): %f", speed)
	}

	// Always create a mono TSM — stereo is handled by downmix/upmix in Process()
	t, err := wsola.Default(1, speed)
	if err != nil {
		return nil, fmt.Errorf("creating WSOLA processor: %w", err)
	}

	return &Processor{
		tsm:   t,
		speed: speed,
	}, nil
}

// Process takes int16 PCM audio bytes and returns time-stretched int16 PCM bytes
// and the output sample count (per channel).
//
// For stereo input: downmixes to mono, runs WSOLA, duplicates back to stereo.
// For mono input: runs WSOLA directly.
//
// bytesPerSample must be 2 (int16). If 3 (24-bit), input is truncated to 16-bit.
func (tp *Processor) Process(input []byte, channels, bytesPerSample int) ([]byte, int) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if bytesPerSample == 3 {
		// 24-bit: truncate to 16-bit for WSOLA (tempo change is lossy anyway)
		input = truncate24to16(input)
		bytesPerSample = 2
	}
	if bytesPerSample != 2 {
		slog.Warn("tempo.Processor: unsupported bytes per sample, passing through", "bps", bytesPerSample)
		return input, len(input) / (channels * bytesPerSample)
	}

	totalSamples := len(input) / 2 // total int16 values
	samplesPerChannel := totalSamples / channels

	if samplesPerChannel == 0 {
		return input, 0
	}

	// Convert int16 PCM → mono float64 buffer
	monoBuf := multichannel.NewTSMBuffer(1, samplesPerChannel)
	if channels == 1 {
		for i := 0; i < samplesPerChannel; i++ {
			idx := i * 2
			val := int16(binary.LittleEndian.Uint16(input[idx : idx+2]))
			monoBuf[0][i] = float64(val) / 32768.0
		}
	} else {
		// Stereo → mono: average L and R
		for i := 0; i < samplesPerChannel; i++ {
			idxL := (i*channels + 0) * 2
			idxR := (i*channels + 1) * 2
			valL := int16(binary.LittleEndian.Uint16(input[idxL : idxL+2]))
			valR := int16(binary.LittleEndian.Uint16(input[idxR : idxR+2]))
			monoBuf[0][i] = (float64(valL) + float64(valR)) / (2.0 * 32768.0)
		}
	}

	// Feed mono input in chunks sized by RemainingInputSpace(), collect output.
	const maxOutputPerChunk = 4096
	var outputChunks []multichannel.TSMBuffer
	var outputLengths []int
	totalOutput := 0

	inputPos := 0
	for inputPos < samplesPerChannel {
		space := tp.tsm.RemainingInputSpace()
		if space <= 0 {
			outBuf := multichannel.NewTSMBuffer(1, maxOutputPerChunk)
			nOut := tp.tsm.Receive(outBuf)
			if nOut > 0 {
				outputChunks = append(outputChunks, outBuf)
				outputLengths = append(outputLengths, nOut)
				totalOutput += nOut
			}
			space = tp.tsm.RemainingInputSpace()
			if space <= 0 {
				slog.Warn("tempo.Processor: TSM has no input space after receive",
					"inputPos", inputPos, "total", samplesPerChannel)
				break
			}
		}

		end := inputPos + space
		if end > samplesPerChannel {
			end = samplesPerChannel
		}

		chunk := monoBuf.Slice(inputPos, end)
		tp.tsm.Put(chunk)
		inputPos = end

		outBuf := multichannel.NewTSMBuffer(1, maxOutputPerChunk)
		nOut := tp.tsm.Receive(outBuf)
		if nOut > 0 {
			outputChunks = append(outputChunks, outBuf)
			outputLengths = append(outputLengths, nOut)
			totalOutput += nOut
		}
	}

	if totalOutput == 0 {
		return nil, 0
	}

	// Convert mono float64 output → int16 PCM (same channel count as input)
	outBytes := make([]byte, totalOutput*channels*2)
	outPos := 0
	for idx, chunk := range outputChunks {
		nOut := outputLengths[idx]
		for i := 0; i < nOut; i++ {
			val := chunk[0][i]
			if val > 1.0 {
				val = 1.0
			} else if val < -1.0 {
				val = -1.0
			}
			sample := int16(val * 32767.0)
			for ch := 0; ch < channels; ch++ {
				binary.LittleEndian.PutUint16(outBytes[outPos:outPos+2], uint16(sample))
				outPos += 2
			}
		}
	}

	return outBytes, totalOutput
}

// SetSpeed changes playback speed dynamically. No restart needed.
func (tp *Processor) SetSpeed(speed float64) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.speed = speed
	tp.tsm.SetSpeed(speed)
}

// Speed returns the current playback speed.
func (tp *Processor) Speed() float64 {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	return tp.speed
}

// Clear resets the processor state (e.g., when switching chapters).
func (tp *Processor) Clear() {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.tsm.Clear()
}

// truncate24to16 converts 24-bit LE PCM samples to 16-bit LE PCM by taking the
// upper 2 bytes of each 3-byte sample (equivalent to >> 8).
func truncate24to16(input []byte) []byte {
	sampleCount := len(input) / 3
	out := make([]byte, sampleCount*2)
	for i := 0; i < sampleCount; i++ {
		out[i*2] = input[i*3+1]
		out[i*2+1] = input[i*3+2]
	}
	return out
}
