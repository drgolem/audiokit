package opus

import (
	"fmt"

	goopus "github.com/drgolem/go-opus/opus"
	"github.com/drgolem/ringbuffer"
)

// Decoder wraps go-opus OpusFileDecoder for OGG Opus file decoding.
// Pure Go implementation — no CGo required.
// Implements decoder.AudioDecoder.
type Decoder struct {
	decoder    *goopus.OpusFileDecoder
	ringBuffer *ringbuffer.RingBuffer
	channels   int
}

// NewDecoder creates a new Opus decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) Open(fileName string) error {
	dec, err := goopus.NewOpusFileDecoder(fileName)
	if err != nil {
		return fmt.Errorf("failed to open Opus file %s: %w", fileName, err)
	}
	d.decoder = dec
	d.channels = dec.Channels()

	samplesReq := 4096 * 2
	d.ringBuffer = ringbuffer.New(uint64(2 * d.channels * samplesReq))

	return nil
}

func (d *Decoder) Close() error {
	if d.decoder != nil {
		d.decoder.Close()
		d.decoder = nil
	}
	return nil
}

func (d *Decoder) GetFormat() (sampleRate, channels, bitsPerSample int) {
	if d.decoder == nil {
		return 0, 0, 0
	}
	return d.decoder.SampleRate(), d.channels, 16
}

func (d *Decoder) DecodeSamples(samples int, audio []byte) (int, error) {
	if d.decoder == nil {
		return 0, fmt.Errorf("decoder not initialized")
	}

	outputBytesPerSample := 2
	for {
		sampleBytes := d.ringBuffer.Size()
		samplesAvail := int(sampleBytes / uint64(d.channels*outputBytesPerSample))
		if samplesAvail >= samples {
			bytesRequest := samples * d.channels * outputBytesPerSample
			bytesRead, err := d.ringBuffer.Read(audio[:bytesRequest])
			if err != nil {
				return 0, err
			}
			return bytesRead / (d.channels * outputBytesPerSample), nil
		}

		const decodeBatch = 2048
		out := make([]byte, decodeBatch*d.channels*2)
		nSamples, err := d.decoder.DecodeSamples(decodeBatch, out)
		if err != nil {
			return 0, err
		}
		if nSamples == 0 {
			// EOF — return whatever is buffered
			avail := d.ringBuffer.Size()
			if avail == 0 {
				return 0, nil
			}
			bytesRead, err := d.ringBuffer.Read(audio[:avail])
			if err != nil {
				return 0, err
			}
			return bytesRead / (d.channels * outputBytesPerSample), nil
		}

		bytesLen := nSamples * d.channels * 2
		if _, err := d.ringBuffer.Write(out[:bytesLen]); err != nil {
			return 0, err
		}
	}
}
