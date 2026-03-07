package stream

import (
	"context"
	"io"
	"testing"

	"github.com/drgolem/audiokit/pkg/decoder"
)

// Verify StreamDecoder implements decoder.AudioDecoder at compile time.
var _ decoder.AudioDecoder = (*StreamDecoder)(nil)

// mockProvider is a test AudioPacketProvider that returns a fixed sequence of packets.
type mockProvider struct {
	packets []AudioPacket
	idx     int
}

func (m *mockProvider) ReadAudioPacket(ctx context.Context, samples int) (*AudioPacket, error) {
	if m.idx >= len(m.packets) {
		return nil, io.EOF
	}
	pkt := &m.packets[m.idx]
	m.idx++
	return pkt, nil
}

func TestStreamDecoderBasic(t *testing.T) {
	format := AudioFormat{SampleRate: 44100, Channels: 2, BytesPerSample: 2}
	audio := []byte{0x01, 0x02, 0x03, 0x04}
	provider := &mockProvider{
		packets: []AudioPacket{
			{Format: format, SamplesCount: 1, Audio: audio},
		},
	}

	dec := NewStreamDecoder(context.Background(), provider, format)

	// Check format
	rate, ch, bps := dec.GetFormat()
	if rate != 44100 || ch != 2 || bps != 16 {
		t.Errorf("GetFormat: got %d/%d/%d, want 44100/2/16", rate, ch, bps)
	}

	// Decode
	buf := make([]byte, 64)
	n, err := dec.DecodeSamples(1, buf)
	if err != nil {
		t.Fatalf("DecodeSamples: %v", err)
	}
	if n != 1 {
		t.Errorf("DecodeSamples: got %d samples, want 1", n)
	}
	if buf[0] != 0x01 || buf[1] != 0x02 || buf[2] != 0x03 || buf[3] != 0x04 {
		t.Errorf("DecodeSamples: audio data mismatch")
	}

	// EOF
	_, err = dec.DecodeSamples(1, buf)
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
}

func TestStreamDecoderFormatChange(t *testing.T) {
	format1 := AudioFormat{SampleRate: 44100, Channels: 2, BytesPerSample: 2}
	format2 := AudioFormat{SampleRate: 48000, Channels: 1, BytesPerSample: 2}

	provider := &mockProvider{
		packets: []AudioPacket{
			{Format: format1, SamplesCount: 1, Audio: make([]byte, 4)},
			{Format: format2, SamplesCount: 1, Audio: make([]byte, 2)},
		},
	}

	dec := NewStreamDecoder(context.Background(), provider, format1)
	buf := make([]byte, 64)

	// First packet — no format change
	_, err := dec.DecodeSamples(1, buf)
	if err != nil {
		t.Fatalf("DecodeSamples: %v", err)
	}

	select {
	case <-dec.FormatChanges():
		t.Error("Unexpected format change on first packet")
	default:
	}

	// Second packet — format changes
	_, err = dec.DecodeSamples(1, buf)
	if err != nil {
		t.Fatalf("DecodeSamples: %v", err)
	}

	select {
	case newFmt := <-dec.FormatChanges():
		if newFmt.SampleRate != 48000 || newFmt.Channels != 1 {
			t.Errorf("Format change: got %+v, want 48000/1", newFmt)
		}
	default:
		t.Error("Expected format change notification")
	}

	// Verify GetFormat reflects the change
	rate, ch, _ := dec.GetFormat()
	if rate != 48000 || ch != 1 {
		t.Errorf("GetFormat after change: got %d/%d, want 48000/1", rate, ch)
	}
}

func TestStreamDecoderEmptyPacket(t *testing.T) {
	format := AudioFormat{SampleRate: 44100, Channels: 2, BytesPerSample: 2}
	provider := &mockProvider{
		packets: []AudioPacket{
			{Format: format, SamplesCount: 0, Audio: nil},
		},
	}

	dec := NewStreamDecoder(context.Background(), provider, format)
	buf := make([]byte, 64)

	n, err := dec.DecodeSamples(1, buf)
	if err != nil {
		t.Fatalf("DecodeSamples: %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 samples for empty packet, got %d", n)
	}
}

func TestStreamDecoderOpenCloseNoop(t *testing.T) {
	format := AudioFormat{SampleRate: 44100, Channels: 2, BytesPerSample: 2}
	dec := NewStreamDecoder(context.Background(), &mockProvider{}, format)

	if err := dec.Open("anything"); err != nil {
		t.Errorf("Open: %v", err)
	}
	if err := dec.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
