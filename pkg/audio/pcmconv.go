package audio

// PCM24to16 converts 24-bit little-endian PCM audio to 16-bit little-endian PCM.
// For each 3-byte sample, it drops the least-significant byte (byte 0) and keeps
// the upper two bytes (bytes 1-2), which is equivalent to a right-shift by 8 bits.
// Returns a new slice containing the 16-bit data.
func PCM24to16(audio []byte) []byte {
	samples := len(audio) / 3
	out := make([]byte, samples*2)
	for i := 0; i < samples; i++ {
		out[i*2] = audio[i*3+1]
		out[i*2+1] = audio[i*3+2]
	}
	return out
}
