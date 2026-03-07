package types

import "fmt"

// FrameFormat describes the PCM audio sample format.
//
// This is the canonical definition, shared across all learnAudio projects.
// Previously duplicated in sonet (3 versions), musiclab, and musictools.
type FrameFormat struct {
	SampleRate    int
	Channels      int
	BitsPerSample int // 8, 16, 24, 32
}

// BytesPerSample returns the number of bytes per single-channel sample.
func (f FrameFormat) BytesPerSample() int {
	return f.BitsPerSample / 8
}

// FrameSize returns the number of bytes per complete frame (all channels, one sample period).
func (f FrameFormat) FrameSize() int {
	return f.Channels * f.BytesPerSample()
}

func (f FrameFormat) String() string {
	return fmt.Sprintf("%d:%d:%d", f.SampleRate, f.Channels, f.BitsPerSample)
}

// SampleFormat describes the PCM sample encoding as an enum.
// Useful when the exact encoding matters (e.g., float vs int).
type SampleFormat int

const (
	SampleFmtInt8 SampleFormat = iota + 1
	SampleFmtInt16
	SampleFmtInt24
	SampleFmtInt32
	SampleFmtFloat32
)

// SampleFormatFromBitsPerSample converts a bits-per-sample value to SampleFormat.
// Defaults to SampleFmtInt16 for unrecognized values.
func SampleFormatFromBitsPerSample(bps int) SampleFormat {
	switch bps {
	case 8:
		return SampleFmtInt8
	case 16:
		return SampleFmtInt16
	case 24:
		return SampleFmtInt24
	case 32:
		return SampleFmtInt32
	default:
		return SampleFmtInt16
	}
}

// BytesPerSample returns the byte width for this sample format.
func (f SampleFormat) BytesPerSample() int {
	switch f {
	case SampleFmtInt8:
		return 1
	case SampleFmtInt16:
		return 2
	case SampleFmtInt24:
		return 3
	case SampleFmtInt32, SampleFmtFloat32:
		return 4
	default:
		return 0
	}
}

// FileFormatType identifies an audio file format by extension.
type FileFormatType string

const (
	FileFormatMP3  FileFormatType = ".mp3"
	FileFormatFLAC FileFormatType = ".flac"
	FileFormatOGG  FileFormatType = ".ogg"
	FileFormatWAV  FileFormatType = ".wav"
	FileFormatCUE  FileFormatType = ".cue"
)
