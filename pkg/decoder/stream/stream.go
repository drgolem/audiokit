// Package stream provides a streaming adapter for decoder.AudioDecoder.
//
// StreamDecoder wraps any AudioPacketProvider (network streams, shared buffers,
// etc.) and presents it as a standard AudioDecoder. This allows streaming sources
// to be used anywhere a file decoder would be used.
//
// AudioPacketProvider is the key interface — implement it to plug any audio
// source into the decoder pipeline.
package stream

import (
	"context"
	"sync"
)

// AudioFormat describes the audio stream format.
// Uses int fields (not compact uint types) since this is an in-memory
// streaming type, not a serialized wire format.
type AudioFormat struct {
	SampleRate     int
	Channels       int
	BytesPerSample int
}

// AudioPacket represents a chunk of decoded audio data with format metadata.
type AudioPacket struct {
	CreateTs     int64 // Creation timestamp (nanoseconds, 0 if unused)
	Format       AudioFormat
	SamplesCount int
	Audio        []byte
}

// AudioPacketProvider is the interface for sources that provide audio data.
// Implement this to connect any audio source (network streams, shared buffers,
// inter-process pipes, etc.) into the decoder pipeline.
//
// ReadAudioPacket reads the next audio packet with up to `samples` samples.
// Returns io.EOF when the stream ends.
type AudioPacketProvider interface {
	ReadAudioPacket(ctx context.Context, samples int) (*AudioPacket, error)
}

// StreamDecoder adapts an AudioPacketProvider to the decoder.AudioDecoder interface.
// It handles format detection and dynamic format changes during streaming.
//
// Thread safety:
//   - GetFormat() is safe to call concurrently with DecodeSamples()
//   - DecodeSamples() should be called from a single goroutine
type StreamDecoder struct {
	provider     AudioPacketProvider
	format       AudioFormat
	formatMx     sync.RWMutex
	formatChange chan AudioFormat
	ctx          context.Context
}

// NewStreamDecoder creates a decoder for streaming audio sources.
func NewStreamDecoder(ctx context.Context, provider AudioPacketProvider, initialFormat AudioFormat) *StreamDecoder {
	return &StreamDecoder{
		provider:     provider,
		format:       initialFormat,
		formatChange: make(chan AudioFormat, 1),
		ctx:          ctx,
	}
}

// Open is a no-op for stream decoders — the source is provided at construction.
func (d *StreamDecoder) Open(fileName string) error {
	return nil
}

// Close is a no-op — the caller owns the AudioPacketProvider lifecycle.
func (d *StreamDecoder) Close() error {
	return nil
}

// GetFormat returns the current audio format (sample rate, channels, bits per sample).
// Safe for concurrent access.
func (d *StreamDecoder) GetFormat() (rate, channels, bitsPerSample int) {
	d.formatMx.RLock()
	defer d.formatMx.RUnlock()
	return d.format.SampleRate,
		d.format.Channels,
		d.format.BytesPerSample * 8
}

// DecodeSamples reads the next packet from the provider and copies audio data
// into the provided buffer. Detects and signals format changes.
func (d *StreamDecoder) DecodeSamples(samples int, audio []byte) (int, error) {
	pkt, err := d.provider.ReadAudioPacket(d.ctx, samples)
	if err != nil {
		return 0, err
	}

	if pkt.SamplesCount == 0 {
		return 0, nil
	}

	// Check for format change
	if d.formatChanged(pkt.Format) {
		d.formatMx.Lock()
		d.format = pkt.Format
		d.formatMx.Unlock()

		// Signal format change (non-blocking)
		select {
		case d.formatChange <- pkt.Format:
		default:
		}
	}

	// Copy audio data
	bytesToCopy := pkt.SamplesCount * pkt.Format.Channels * pkt.Format.BytesPerSample
	copy(audio, pkt.Audio[:bytesToCopy])

	return pkt.SamplesCount, nil
}

func (d *StreamDecoder) formatChanged(newFormat AudioFormat) bool {
	d.formatMx.RLock()
	defer d.formatMx.RUnlock()

	return d.format.SampleRate != newFormat.SampleRate ||
		d.format.Channels != newFormat.Channels ||
		d.format.BytesPerSample != newFormat.BytesPerSample
}

// FormatChanges returns a channel that receives format change notifications.
// Buffered (capacity 1) — if the consumer is slow, intermediate changes are dropped.
func (d *StreamDecoder) FormatChanges() <-chan AudioFormat {
	return d.formatChange
}
