package decoder

import "testing"

// Verify interface compliance at compile time.
// Concrete implementations in separate codec packages will assert this.
func TestAudioDecoderInterface(t *testing.T) {
	// AudioDecoder is the base interface
	var _ AudioDecoder = (Seekable)(nil)
	// Seekable embeds AudioDecoder
	t.Log("Seekable satisfies AudioDecoder")
}

func TestRegistrySupports(t *testing.T) {
	r := NewRegistry()
	r.Register(".mp3", func(int) (AudioDecoder, error) { return nil, nil })
	r.Register(".flac", func(int) (AudioDecoder, error) { return nil, nil })

	if !r.Supports(".mp3") {
		t.Error("should support .mp3")
	}
	if !r.Supports(".MP3") {
		t.Error("should support .MP3 (case-insensitive)")
	}
	if r.Supports(".wav") {
		t.Error("should not support .wav (not registered)")
	}
}

func TestRegistryExtensions(t *testing.T) {
	r := NewRegistry()
	r.Register(".mp3", func(int) (AudioDecoder, error) { return nil, nil })
	r.Register(".flac", func(int) (AudioDecoder, error) { return nil, nil })

	exts := r.Extensions()
	if len(exts) != 2 {
		t.Errorf("expected 2 extensions, got %d", len(exts))
	}
}

func TestRegistryNewFromFile_Unsupported(t *testing.T) {
	r := NewRegistry()
	_, err := r.NewFromFile("test.xyz", 0)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}
