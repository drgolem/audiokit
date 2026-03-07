package vorbis

import (
	"errors"
	"io"
	"math"
	"os"

	"github.com/drgolem/ringbuffer"
	"github.com/jfreymuth/oggvorbis"
)

var errNoDecoder = errors.New("decoder not initialized")

// Decoder wraps jfreymuth/oggvorbis for OGG Vorbis file decoding.
// Pure Go implementation — no CGo required.
// Implements decoder.AudioDecoder and decoder.Seekable.
type Decoder struct {
	file                 *os.File
	reader               *oggvorbis.Reader
	bitsPerSample        int
	outputBytesPerSample int
	channels             int
	currSample           int64
	ringBuffer           *ringbuffer.RingBuffer
	decodeBuffer         []float32
}

const framesPerBuffer = 4096

// NewDecoder creates a new Vorbis decoder.
// sampleBitDepth is the desired output bit depth (0 = 16-bit default).
func NewDecoder(sampleBitDepth int) (*Decoder, error) {
	if sampleBitDepth == 0 {
		sampleBitDepth = 16
	}

	const bufSize = 2 * 2 * 4 * framesPerBuffer

	return &Decoder{
		bitsPerSample:        sampleBitDepth,
		outputBytesPerSample: sampleBitDepth / 8,
		ringBuffer:           ringbuffer.New(uint64(bufSize)),
		decodeBuffer:         make([]float32, 2*framesPerBuffer),
	}, nil
}

func (d *Decoder) Open(fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	d.file = f

	reader, err := oggvorbis.NewReader(io.ReadSeeker(f))
	if err != nil {
		f.Close()
		return err
	}
	d.reader = reader
	d.channels = reader.Channels()

	return nil
}

func (d *Decoder) Close() error {
	d.reader = nil
	if d.file != nil {
		err := d.file.Close()
		d.file = nil
		return err
	}
	return nil
}

func (d *Decoder) GetFormat() (sampleRate, channels, bitsPerSample int) {
	if d.reader == nil {
		return 0, 0, 0
	}
	return d.reader.SampleRate(), d.channels, d.bitsPerSample
}

func (d *Decoder) DecodeSamples(samples int, audio []byte) (int, error) {
	if d.reader == nil {
		return 0, errNoDecoder
	}

	var err error
	for {
		sampleBytes := d.ringBuffer.Size()
		samplesAvail := int(sampleBytes / uint64(d.channels*d.outputBytesPerSample))
		if err == io.EOF || samplesAvail >= samples {
			bytesRequest := samples * d.channels * d.outputBytesPerSample
			if err == io.EOF {
				bytesRequest = int(d.ringBuffer.Size())
			}
			if bytesRequest == 0 {
				return 0, nil
			}

			bytesRead, readErr := d.ringBuffer.Read(audio[:bytesRequest])
			samplesRead := bytesRead / (d.channels * d.outputBytesPerSample)
			d.currSample += int64(samplesRead)
			return samplesRead, readErr
		}

		samplesRead, readErr := d.reader.Read(d.decodeBuffer)
		if readErr == io.EOF {
			err = io.EOF
			continue
		}
		if readErr != nil {
			return 0, readErr
		}

		// Convert interleaved float32 to int16 PCM little-endian
		var b16 [2]byte
		for idx := 0; idx < samplesRead; {
			for j := 0; j < d.channels; j++ {
				sv := int16(math.Floor(float64(d.decodeBuffer[idx+j]) * 32767))
				b16[0] = byte(sv & 0xFF)
				b16[1] = byte(sv >> 8)
				if _, writeErr := d.ringBuffer.Write(b16[:2]); writeErr != nil {
					return 0, writeErr
				}
			}
			idx += d.channels
		}
	}
}

// Seek seeks relative to the current position (whence is ignored — always relative).
func (d *Decoder) Seek(offset int64, whence int) (int64, error) {
	if d.reader == nil {
		return 0, errNoDecoder
	}

	seekSample := d.currSample + offset
	if err := d.reader.SetPosition(seekSample); err != nil {
		return 0, err
	}

	d.ringBuffer.Reset()
	d.currSample = seekSample

	return seekSample, nil
}

// TellCurrentSample returns the current decoder position in sample frames.
func (d *Decoder) TellCurrentSample() int64 {
	return d.currSample
}
