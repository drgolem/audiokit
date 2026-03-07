package mp3

import (
	"errors"
	"io"
	"os"

	gomp3 "github.com/imcarsen/go-mp3"
)

// bytesPerSampleFrame is the number of bytes per sample frame (stereo 16-bit).
// imcarsen/go-mp3 always outputs stereo interleaved signed 16-bit PCM.
const bytesPerSampleFrame = 4

var errNoDecoder = errors.New("decoder not initialized")

// Decoder wraps imcarsen/go-mp3 to provide MP3 decoding.
// Pure Go implementation — no CGo required.
// Always outputs stereo 16-bit PCM regardless of source format.
// Implements decoder.AudioDecoder and decoder.Seekable.
type Decoder struct {
	decoder  *gomp3.Decoder
	file     *os.File
	bytesPos int64
}

// NewDecoder creates a new MP3 decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) Open(fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}

	dec, err := gomp3.NewDecoder(f)
	if err != nil {
		f.Close()
		return err
	}

	d.file = f
	d.decoder = dec
	d.bytesPos = 0
	return nil
}

func (d *Decoder) Close() error {
	d.decoder = nil
	if d.file != nil {
		err := d.file.Close()
		d.file = nil
		return err
	}
	return nil
}

func (d *Decoder) GetFormat() (sampleRate, channels, bitsPerSample int) {
	if d.decoder == nil {
		return 0, 0, 0
	}
	return d.decoder.SampleRate(), 2, 16
}

func (d *Decoder) DecodeSamples(samples int, audio []byte) (int, error) {
	if d.decoder == nil {
		return 0, errNoDecoder
	}

	bytesNeeded := samples * bytesPerSampleFrame
	if bytesNeeded > len(audio) {
		bytesNeeded = len(audio)
	}

	totalRead := 0
	for totalRead < bytesNeeded {
		n, err := d.decoder.Read(audio[totalRead:bytesNeeded])
		totalRead += n
		d.bytesPos += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return totalRead / bytesPerSampleFrame, err
		}
	}

	return totalRead / bytesPerSampleFrame, nil
}

// Seek seeks to the given sample offset.
func (d *Decoder) Seek(offset int64, whence int) (int64, error) {
	if d.decoder == nil {
		return 0, errNoDecoder
	}

	byteOffset := offset * bytesPerSampleFrame
	pos, err := d.decoder.Seek(byteOffset, whence)
	if err != nil {
		return 0, err
	}
	d.bytesPos = pos
	return pos / bytesPerSampleFrame, nil
}

// TellCurrentSample returns the current decoder position in sample frames.
func (d *Decoder) TellCurrentSample() int64 {
	return d.bytesPos / bytesPerSampleFrame
}
