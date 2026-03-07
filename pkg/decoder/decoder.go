package decoder

import "io"

// AudioDecoder is the unified interface for audio file decoders.
//
// All codec implementations (MP3, FLAC, WAV, Vorbis, Opus) implement this
// interface to provide a consistent API for decoding audio files to raw PCM.
//
// Previously duplicated as:
//   - sonet decoders.AudioFrameDecoder
//   - musictools types.AudioDecoder
//   - musiclab audiosource.musicDecoder (unexported)
type AudioDecoder interface {
	// Open opens an audio file for decoding.
	Open(fileName string) error

	// Close releases decoder resources.
	Close() error

	// GetFormat returns the audio format: sample rate (Hz), channels, bits per sample.
	GetFormat() (sampleRate, channels, bitsPerSample int)

	// DecodeSamples decodes up to `samples` sample frames into the provided buffer.
	// The buffer must be at least samples * channels * (bitsPerSample/8) bytes.
	// Returns the number of sample frames actually decoded.
	DecodeSamples(samples int, audio []byte) (int, error)
}

// Seekable extends AudioDecoder with seek and position tracking.
// Only some codecs support seeking (FLAC, MP3). OGG Vorbis file decoder
// supports it via SetPosition; packet-level decoders typically do not.
type Seekable interface {
	AudioDecoder
	io.Seeker

	// TellCurrentSample returns the current decoder position in sample frames.
	TellCurrentSample() int64
}
