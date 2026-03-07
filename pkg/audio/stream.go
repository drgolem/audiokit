package audio

import (
	"github.com/drgolem/audiokit/pkg/types"
)

// AudioStream is the producer interface for decoded audio data.
// Implementations push audio packets onto a channel for consumption
// by sinks (playback, streaming, analysis).
//
// Previously duplicated identically in sonet and musiclab.
type AudioStream interface {
	// GetFormat returns the audio format of this stream.
	GetFormat() types.FrameFormat

	// Status returns key-value pairs describing the stream state.
	Status() map[string]string

	// Stream returns the channel that delivers audio packets.
	// The channel is closed when the stream ends (EOF or error).
	Stream() <-chan AudioSamplesPacket

	// Close stops the stream and releases resources.
	Close() error
}

// AudioSamplesPacket carries a chunk of decoded PCM audio data.
type AudioSamplesPacket struct {
	// CreateTs is the creation timestamp (nanoseconds, monotonic).
	// Used for latency tracking in streaming pipelines. Zero if unused.
	CreateTs int64

	// Format describes the PCM format of the Audio data.
	Format types.FrameFormat

	// SamplesCount is the number of sample frames in this packet.
	// One frame = one sample per channel.
	SamplesCount int

	// Audio is the raw PCM data.
	// Size = SamplesCount * Format.Channels * Format.BytesPerSample()
	Audio []byte
}
