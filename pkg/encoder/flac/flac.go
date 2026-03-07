package flac

import (
	"fmt"

	goflac "github.com/drgolem/go-flac/flac"
)

// BlockSize is the number of samples per FLAC frame for streaming.
// 4096 is libFLAC's default and provides a good balance between
// compression ratio and latency (~93ms at 44.1kHz, ~85ms at 48kHz).
const BlockSize = 4096

// Encoder wraps go-flac's FlacEncoder for real-time streaming.
// It accepts raw PCM bytes and produces encoded FLAC frame bytes.
type Encoder struct {
	enc           *goflac.FlacEncoder
	sampleRate    int
	channels      int
	bitsPerSample int
	int32Buf      []int32 // reusable PCM→int32 conversion buffer
}

// NewEncoder creates a new FLAC encoder for streaming use.
// Uses stream mode (InitStream) so encoded bytes are collected via
// write callbacks and returned from Encode().
func NewEncoder(sampleRate, channels, bitsPerSample int) (*Encoder, error) {
	enc, err := goflac.NewFlacEncoder(sampleRate, channels, bitsPerSample)
	if err != nil {
		return nil, fmt.Errorf("failed to create FLAC encoder: %w", err)
	}

	if err := enc.InitStream(); err != nil {
		enc.Close()
		return nil, fmt.Errorf("failed to init FLAC stream encoder: %w", err)
	}

	return &Encoder{
		enc:           enc,
		sampleRate:    sampleRate,
		channels:      channels,
		bitsPerSample: bitsPerSample,
		int32Buf:      make([]int32, BlockSize*channels),
	}, nil
}

// Encode takes raw PCM bytes (interleaved, little-endian) and returns
// encoded FLAC frame bytes. May return nil if the encoder hasn't produced
// a complete frame yet (still buffering internally).
func (e *Encoder) Encode(audio []byte) ([]byte, error) {
	bytesPerSample := e.bitsPerSample / 8
	totalSamples := len(audio) / (bytesPerSample * e.channels)
	if totalSamples == 0 {
		return nil, nil
	}

	// Ensure int32 buffer is large enough
	needed := totalSamples * e.channels
	if len(e.int32Buf) < needed {
		e.int32Buf = make([]int32, needed)
	}

	// Convert PCM bytes to int32 samples
	goflac.PCMToInt32(audio[:totalSamples*e.channels*bytesPerSample], e.bitsPerSample, e.int32Buf[:needed])

	// Feed to encoder
	if err := e.enc.ProcessInterleaved(e.int32Buf[:needed], totalSamples); err != nil {
		return nil, fmt.Errorf("FLAC encode failed: %w", err)
	}

	// Retrieve any encoded bytes produced by the write callback
	return e.enc.TakeBytes(), nil
}

// Flush finalizes the encoder and returns any remaining encoded bytes.
// After Flush, the encoder cannot be used for further encoding.
func (e *Encoder) Flush() ([]byte, error) {
	if e.enc == nil {
		return nil, nil
	}

	if err := e.enc.Finish(); err != nil {
		return nil, fmt.Errorf("FLAC encoder finish failed: %w", err)
	}

	return e.enc.TakeBytes(), nil
}

// StreamInfo returns the raw STREAMINFO metadata block (34 bytes).
// Only valid after at least one Encode() call (the encoder needs to
// process some audio before STREAMINFO is finalized).
// Note: Full STREAMINFO with accurate MD5 is only available after Flush().
func (e *Encoder) StreamInfo() []byte {
	if e.enc == nil {
		return nil
	}
	return e.enc.StreamInfo()
}

// GetFormat returns the encoder's configured audio format.
func (e *Encoder) GetFormat() (rate, channels, bitsPerSample int) {
	return e.sampleRate, e.channels, e.bitsPerSample
}

// Close releases all resources held by the encoder.
func (e *Encoder) Close() {
	if e.enc != nil {
		e.enc.Close()
		e.enc = nil
	}
}
