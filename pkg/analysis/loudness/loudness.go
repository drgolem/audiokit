package loudness

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"

	"github.com/drgolem/audiokit/pkg/decoder"
	"github.com/drgolem/audiokit/pkg/types"
	"github.com/exaring/ebur128"
	soxr "github.com/zaf/resample"
)

const (
	// ReferenceLevel is the EBU R128 / ReplayGain 2.0 target loudness.
	ReferenceLevel = -18.0 // LUFS

	// framesPerChunk is the number of audio frames decoded per iteration.
	framesPerChunk = 4096

	// analysisSampleRate is the rate required by the ebur128 library.
	analysisSampleRate = 48000

	// analysisBitDepth forces all decoders to output int16 (2 bytes per sample).
	analysisBitDepth = 16
)

// soxrMu serializes soxr_create calls — libsoxr is not thread-safe during init.
var soxrMu sync.Mutex

// DecoderFactory creates a decoder for the given file at the specified bit depth.
// The returned decoder must already be opened (ready to call DecodeSamples).
type DecoderFactory func(fileName string, bitDepth int) (decoder.AudioDecoder, error)

// Analyzer computes EBU R128 loudness for an audio file.
// Each instance should be used by a single goroutine.
type Analyzer struct {
	newDecoder DecoderFactory
}

// NewAnalyzer creates a loudness analyzer using the provided decoder factory.
func NewAnalyzer(factory DecoderFactory) *Analyzer {
	return &Analyzer{newDecoder: factory}
}

// Analyze fully decodes an audio file and returns computed ReplayGain values.
func (a *Analyzer) Analyze(fileName string) (*types.ReplayGainValues, error) {
	dec, err := a.newDecoder(fileName, analysisBitDepth)
	if err != nil {
		return nil, fmt.Errorf("open decoder: %w", err)
	}
	defer dec.Close()

	sampleRate, channels, _ := dec.GetFormat()
	if sampleRate == 0 || channels == 0 {
		return nil, fmt.Errorf("invalid format: sampleRate=%d channels=%d", sampleRate, channels)
	}

	meter, err := ebur128.New(ebur128.LayoutStereo, analysisSampleRate)
	if err != nil {
		return nil, fmt.Errorf("create ebur128 meter: %w", err)
	}

	needResample := sampleRate != analysisSampleRate
	isMono := channels == 1
	bytesPerSample := 2 // all decoders output int16 with analysisBitDepth=16
	bytesPerFrame := channels * bytesPerSample
	audioBuf := make([]byte, framesPerChunk*bytesPerFrame)

	totalFrames := 0

	for {
		nFrames, err := dec.DecodeSamples(framesPerChunk, audioBuf)
		if nFrames == 0 {
			break
		}

		pcmBytes := audioBuf[:nFrames*bytesPerFrame]

		if needResample {
			resampled, resErr := resampleTo48k(pcmBytes, sampleRate, channels)
			if resErr != nil {
				return nil, fmt.Errorf("resample: %w", resErr)
			}
			pcmBytes = resampled
			nFrames = len(pcmBytes) / bytesPerFrame
		}

		float64Buf := pcmToFloat64Stereo(pcmBytes, channels, isMono)

		meter.Write(float64Buf)
		totalFrames += nFrames

		if err != nil {
			break
		}
	}

	if totalFrames == 0 {
		return nil, fmt.Errorf("no audio frames decoded")
	}

	meter.Finalize()
	result := meter.Loudness()

	if result.GatedBlockCount == 0 {
		return nil, fmt.Errorf("too short for loudness measurement (no gated blocks)")
	}

	trackGain := ReferenceLevel - result.IntegratedLoudness
	trackPeak := math.Pow(10, result.TruePeak/20.0)

	return &types.ReplayGainValues{
		TrackGain: trackGain,
		TrackPeak: trackPeak,
	}, nil
}

func resampleTo48k(pcmBytes []byte, sourceSR, channels int) ([]byte, error) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	soxrMu.Lock()
	resampler, err := soxr.New(w,
		float64(sourceSR),
		float64(analysisSampleRate),
		channels,
		soxr.I16,
		soxr.HighQ)
	soxrMu.Unlock()
	if err != nil {
		return nil, err
	}

	if _, err := resampler.Write(pcmBytes); err != nil {
		_ = resampler.Close()
		return nil, err
	}

	_ = resampler.Close()
	w.Flush()

	return buf.Bytes(), nil
}

func pcmToFloat64Stereo(pcmBytes []byte, channels int, isMono bool) []float64 {
	nSamples := len(pcmBytes) / 2
	nFrames := nSamples / channels

	out := make([]float64, nFrames*2)

	if isMono {
		for i := 0; i < nFrames; i++ {
			sample := float64(int16(binary.LittleEndian.Uint16(pcmBytes[i*2:]))) / 32768.0
			out[i*2] = sample
			out[i*2+1] = sample
		}
	} else {
		for i := 0; i < nFrames; i++ {
			offset := i * channels * 2
			left := float64(int16(binary.LittleEndian.Uint16(pcmBytes[offset:]))) / 32768.0
			right := float64(int16(binary.LittleEndian.Uint16(pcmBytes[offset+2:]))) / 32768.0
			out[i*2] = left
			out[i*2+1] = right
		}
	}

	return out
}
