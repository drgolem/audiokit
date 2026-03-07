package flac

import (
	"fmt"

	goflac "github.com/drgolem/go-flac/flac"
)

// Decoder wraps go-flac for FLAC decoding.
// Requires CGo (libflac).
// Implements decoder.AudioDecoder and decoder.Seekable.
type Decoder struct {
	decoder  *goflac.FlacDecoder
	rate     int
	channels int
	bps      int
}

// NewDecoder creates a new FLAC decoder.
// sampleBitDepth is the desired output bit depth (0 = codec default, typically 16).
func NewDecoder(sampleBitDepth int) (*Decoder, error) {
	if sampleBitDepth == 0 {
		sampleBitDepth = 16
	}
	return &Decoder{bps: sampleBitDepth}, nil
}

func (d *Decoder) Open(fileName string) error {
	decoder, err := goflac.NewFlacFrameDecoder(d.bps)
	if err != nil {
		return fmt.Errorf("failed to create FLAC decoder: %w", err)
	}

	if err := decoder.Open(fileName); err != nil {
		decoder.Delete()
		return fmt.Errorf("failed to open %s: %w", fileName, err)
	}

	rate, channels, bps := decoder.GetFormat()
	d.decoder = decoder
	d.rate = rate
	d.channels = channels
	d.bps = bps

	return nil
}

func (d *Decoder) Close() error {
	if d.decoder != nil {
		d.decoder.Close()
		d.decoder.Delete()
		d.decoder = nil
	}
	return nil
}

func (d *Decoder) GetFormat() (sampleRate, channels, bitsPerSample int) {
	return d.rate, d.channels, d.bps
}

func (d *Decoder) DecodeSamples(samples int, audio []byte) (int, error) {
	if d.decoder == nil {
		return 0, fmt.Errorf("decoder not initialized")
	}
	return d.decoder.DecodeSamples(samples, audio)
}

// Seek seeks to the given sample offset. Delegates to go-flac's Seek.
func (d *Decoder) Seek(offset int64, whence int) (int64, error) {
	if d.decoder == nil {
		return 0, fmt.Errorf("decoder not initialized")
	}
	return d.decoder.Seek(offset, whence)
}

// TellCurrentSample returns the current decoder position in sample frames.
func (d *Decoder) TellCurrentSample() int64 {
	if d.decoder == nil {
		return 0
	}
	return d.decoder.TellCurrentSample()
}
