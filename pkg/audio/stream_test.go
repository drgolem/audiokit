package audio

import (
	"testing"

	"github.com/drgolem/audiokit/pkg/types"
)

func TestAudioSamplesPacket_Size(t *testing.T) {
	pkt := AudioSamplesPacket{
		Format:       types.FrameFormat{SampleRate: 44100, Channels: 2, BitsPerSample: 16},
		SamplesCount: 1024,
		Audio:        make([]byte, 1024*2*2),
	}

	expectedSize := pkt.SamplesCount * pkt.Format.Channels * pkt.Format.BytesPerSample()
	if len(pkt.Audio) != expectedSize {
		t.Errorf("Audio size = %d, expected %d", len(pkt.Audio), expectedSize)
	}
}
