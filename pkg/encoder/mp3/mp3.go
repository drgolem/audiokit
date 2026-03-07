package mp3

import (
	"bytes"
	"encoding/binary"
	"fmt"

	shine "github.com/braheezy/shine-mp3/pkg/mp3"
)

// SampleRate is the target sample rate for MP3 encoding.
// 44100 Hz is the most universally compatible MP3 sample rate.
const SampleRate = 44100

// SamplesPerFrame is the number of samples per MP3 frame (MPEG-I Layer III).
const SamplesPerFrame = shine.SHINE_MAX_SAMPLES // 1152

// Encoder wraps the shine pure-Go MP3 encoder.
type Encoder struct {
	encoder    *shine.Encoder
	sampleRate int
	channels   int
	outBuf     bytes.Buffer
}

// NewEncoder creates a new MP3 encoder using the shine pure-Go library.
// sampleRate and channels must match the PCM input that will be encoded.
// Bitrate is fixed at 128 kbps (shine default).
func NewEncoder(sampleRate, channels int) (*Encoder, error) {
	enc := shine.NewEncoder(sampleRate, channels)
	if enc == nil {
		return nil, fmt.Errorf("failed to create shine MP3 encoder for rate=%d channels=%d", sampleRate, channels)
	}

	return &Encoder{
		encoder:    enc,
		sampleRate: sampleRate,
		channels:   channels,
	}, nil
}

// Encode takes PCM audio bytes (little-endian signed 16-bit interleaved) and
// returns encoded MP3 frame bytes. May return empty bytes if shine hasn't
// accumulated enough samples for a full frame yet.
func (e *Encoder) Encode(audio []byte) ([]byte, error) {
	samples := bytesToInt16LE(audio)
	e.outBuf.Reset()
	err := e.encoder.Write(&e.outBuf, samples)
	if err != nil {
		return nil, fmt.Errorf("MP3 encode failed: %w", err)
	}
	if e.outBuf.Len() == 0 {
		return nil, nil
	}
	out := make([]byte, e.outBuf.Len())
	copy(out, e.outBuf.Bytes())
	return out, nil
}

// GetFormat returns the encoder's audio format.
func (e *Encoder) GetFormat() (rate, channels, bitsPerSample int) {
	return e.sampleRate, e.channels, 16
}

// SamplesPerFrame returns the number of PCM samples per MP3 frame.
func (e *Encoder) GetSamplesPerFrame() int {
	return SamplesPerFrame
}

// bytesToInt16LE converts a byte slice of little-endian signed 16-bit PCM
// samples to a slice of int16.
func bytesToInt16LE(data []byte) []int16 {
	n := len(data) / 2
	samples := make([]int16, n)
	for i := range n {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
	}
	return samples
}
