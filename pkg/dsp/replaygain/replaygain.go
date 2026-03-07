package replaygain

import (
	"encoding/binary"
	"math"
)

const (
	// MaxGainScale is the maximum linear multiplier allowed (matches MPD).
	MaxGainScale = 15.0
	// UnityScaleFP is the Q15 fixed-point representation of 1.0 (no change).
	UnityScaleFP int32 = 1 << 15
)

// CalculateGainScale converts ReplayGain dB values to a linear multiplier.
// gainDB is the ReplayGain tag value (e.g., -7.03), preampDB is user-configured
// pre-amp adjustment, peak is the track/album peak (linear, 0 = unknown).
// preventClip uses peak to cap gain and avoid clipping.
func CalculateGainScale(gainDB, preampDB, peak float64, preventClip bool) float64 {
	scale := math.Pow(10, (gainDB+preampDB)/20.0)
	if scale > MaxGainScale {
		scale = MaxGainScale
	}
	if preventClip && peak > 0 && scale*peak > 1.0 {
		scale = 1.0 / peak
	}
	return scale
}

// ScaleToFixedPoint converts a linear scale factor to Q15 fixed-point int32.
// Returns UnityScaleFP (1<<15) for scale == 1.0.
func ScaleToFixedPoint(scale float64) int32 {
	return int32(scale * float64(UnityScaleFP))
}

// ApplyGainInt16 applies Q15 fixed-point gain to int16 LE PCM samples in-place.
// No-op when scaleFP == UnityScaleFP (scale 1.0).
func ApplyGainInt16(audio []byte, scaleFP int32) {
	for i := 0; i+1 < len(audio); i += 2 {
		sample := int16(binary.LittleEndian.Uint16(audio[i : i+2]))
		scaled := (int32(sample) * scaleFP) >> 15
		if scaled > 32767 {
			scaled = 32767
		}
		if scaled < -32768 {
			scaled = -32768
		}
		binary.LittleEndian.PutUint16(audio[i:i+2], uint16(int16(scaled)))
	}
}

// ApplyGainInt24 applies Q15 fixed-point gain to 24-bit LE PCM samples in-place.
func ApplyGainInt24(audio []byte, scaleFP int32) {
	for i := 0; i+2 < len(audio); i += 3 {
		// 24-bit LE: [low, mid, high] → sign-extend to int32
		val := int32(audio[i]) | int32(audio[i+1])<<8 | int32(int8(audio[i+2]))<<16
		scaled := (int64(val) * int64(scaleFP)) >> 15
		if scaled > 8388607 {
			scaled = 8388607
		}
		if scaled < -8388608 {
			scaled = -8388608
		}
		audio[i] = byte(scaled)
		audio[i+1] = byte(scaled >> 8)
		audio[i+2] = byte(scaled >> 16)
	}
}
