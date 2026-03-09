package loudness

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/drgolem/audiokit/pkg/decoder"
	"github.com/drgolem/audiokit/pkg/types"
	"github.com/exaring/ebur128"
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
			pcmBytes = resampleLinear16(pcmBytes, sampleRate, analysisSampleRate, channels)
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

// resampleLinear16 resamples interleaved int16 PCM using linear interpolation.
// Sufficient for loudness analysis where sample-accurate reconstruction is not required.
func resampleLinear16(pcmBytes []byte, srcRate, dstRate, channels int) []byte {
	srcFrames := len(pcmBytes) / (channels * 2)
	if srcFrames == 0 {
		return nil
	}

	ratio := float64(dstRate) / float64(srcRate)
	dstFrames := int(float64(srcFrames) * ratio)
	out := make([]byte, dstFrames*channels*2)

	for i := 0; i < dstFrames; i++ {
		srcPos := float64(i) / ratio
		idx := int(srcPos)
		frac := srcPos - float64(idx)

		if idx >= srcFrames-1 {
			idx = srcFrames - 1
			frac = 0
		}

		for ch := 0; ch < channels; ch++ {
			s0 := int16(binary.LittleEndian.Uint16(pcmBytes[(idx*channels+ch)*2:]))
			var s1 int16
			if idx+1 < srcFrames {
				s1 = int16(binary.LittleEndian.Uint16(pcmBytes[((idx+1)*channels+ch)*2:]))
			} else {
				s1 = s0
			}
			interpolated := float64(s0) + frac*(float64(s1)-float64(s0))
			sample := int16(math.Round(interpolated))
			binary.LittleEndian.PutUint16(out[(i*channels+ch)*2:], uint16(sample))
		}
	}
	return out
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
