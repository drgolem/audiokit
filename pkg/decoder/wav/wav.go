package wav

import (
	"fmt"
	"os"

	"github.com/youpy/go-wav"
)

// Decoder wraps go-wav for decoding WAV audio files.
// Pure Go implementation — no CGo required.
// Supports 8, 16, 24, and 32-bit PCM formats.
// Implements decoder.AudioDecoder.
type Decoder struct {
	file     *os.File
	reader   *wav.Reader
	rate     int
	channels int
	bps      int
}

// NewDecoder creates a new WAV decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) Open(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("failed to open WAV file: %w", err)
	}

	reader := wav.NewReader(file)
	format, err := reader.Format()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to read WAV format: %w", err)
	}

	if format.AudioFormat != wav.AudioFormatPCM {
		file.Close()
		return fmt.Errorf("unsupported WAV format: %d (only PCM supported)", format.AudioFormat)
	}

	d.file = file
	d.reader = reader
	d.rate = int(format.SampleRate)
	d.channels = int(format.NumChannels)
	d.bps = int(format.BitsPerSample)

	return nil
}

func (d *Decoder) Close() error {
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

func (d *Decoder) GetFormat() (sampleRate, channels, bitsPerSample int) {
	return d.rate, d.channels, d.bps
}

// DecodeSamples decodes up to `samples` audio sample frames into the provided buffer.
// Supports 8, 16, 24, and 32-bit PCM. The buffer must be large enough:
// samples * channels * (bitsPerSample/8) bytes.
func (d *Decoder) DecodeSamples(samples int, audio []byte) (int, error) {
	if d.reader == nil {
		return 0, fmt.Errorf("decoder not initialized")
	}

	bytesPerSample := d.bps / 8
	totalSamples := 0

	for i := 0; i < samples; i++ {
		samplesData, err := d.reader.ReadSamples(1)
		if err != nil {
			return totalSamples, err
		}
		if len(samplesData) == 0 {
			return totalSamples, nil
		}

		for ch := 0; ch < d.channels; ch++ {
			if ch >= len(samplesData[0].Values) {
				break
			}

			value := samplesData[0].Values[ch]
			offset := (totalSamples*d.channels + ch) * bytesPerSample

			if offset+bytesPerSample > len(audio) {
				return totalSamples, nil
			}

			switch d.bps {
			case 8:
				audio[offset] = byte(value)
			case 16:
				audio[offset] = byte(value & 0xFF)
				audio[offset+1] = byte((value >> 8) & 0xFF)
			case 24:
				audio[offset] = byte(value & 0xFF)
				audio[offset+1] = byte((value >> 8) & 0xFF)
				audio[offset+2] = byte((value >> 16) & 0xFF)
			case 32:
				audio[offset] = byte(value & 0xFF)
				audio[offset+1] = byte((value >> 8) & 0xFF)
				audio[offset+2] = byte((value >> 16) & 0xFF)
				audio[offset+3] = byte((value >> 24) & 0xFF)
			default:
				return totalSamples, fmt.Errorf("unsupported bits per sample: %d", d.bps)
			}
		}

		totalSamples++
	}

	return totalSamples, nil
}
