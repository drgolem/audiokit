package types

import "testing"

func TestFrameFormat_BytesPerSample(t *testing.T) {
	tests := []struct {
		bps  int
		want int
	}{
		{8, 1},
		{16, 2},
		{24, 3},
		{32, 4},
	}
	for _, tt := range tests {
		f := FrameFormat{BitsPerSample: tt.bps}
		if got := f.BytesPerSample(); got != tt.want {
			t.Errorf("BytesPerSample(%d) = %d, want %d", tt.bps, got, tt.want)
		}
	}
}

func TestFrameFormat_FrameSize(t *testing.T) {
	f := FrameFormat{SampleRate: 44100, Channels: 2, BitsPerSample: 16}
	if got := f.FrameSize(); got != 4 {
		t.Errorf("FrameSize() = %d, want 4", got)
	}
}

func TestFrameFormat_String(t *testing.T) {
	f := FrameFormat{SampleRate: 44100, Channels: 2, BitsPerSample: 16}
	want := "44100:2:16"
	if got := f.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestSampleFormatFromBitsPerSample(t *testing.T) {
	tests := []struct {
		bps  int
		want SampleFormat
	}{
		{8, SampleFmtInt8},
		{16, SampleFmtInt16},
		{24, SampleFmtInt24},
		{32, SampleFmtInt32},
		{0, SampleFmtInt16},  // default
		{48, SampleFmtInt16}, // unknown defaults to 16
	}
	for _, tt := range tests {
		if got := SampleFormatFromBitsPerSample(tt.bps); got != tt.want {
			t.Errorf("SampleFormatFromBitsPerSample(%d) = %d, want %d", tt.bps, got, tt.want)
		}
	}
}

func TestSampleFormat_BytesPerSample(t *testing.T) {
	tests := []struct {
		fmt  SampleFormat
		want int
	}{
		{SampleFmtInt8, 1},
		{SampleFmtInt16, 2},
		{SampleFmtInt24, 3},
		{SampleFmtInt32, 4},
		{SampleFmtFloat32, 4},
		{SampleFormat(99), 0}, // unknown
	}
	for _, tt := range tests {
		if got := tt.fmt.BytesPerSample(); got != tt.want {
			t.Errorf("SampleFormat(%d).BytesPerSample() = %d, want %d", tt.fmt, got, tt.want)
		}
	}
}
