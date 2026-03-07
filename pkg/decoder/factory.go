package decoder

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ConstructorFn creates a new AudioDecoder instance.
// sampleBitDepth is the desired output bit depth (0 = codec default).
type ConstructorFn func(sampleBitDepth int) (AudioDecoder, error)

// Registry maps file extensions to decoder constructors.
// Use Register() to add codec support and NewFromFile() to create decoders.
type Registry struct {
	codecs map[string]ConstructorFn
}

// NewRegistry creates an empty decoder registry.
func NewRegistry() *Registry {
	return &Registry{codecs: make(map[string]ConstructorFn)}
}

// Register adds a decoder constructor for the given file extension (e.g. ".mp3").
func (r *Registry) Register(ext string, fn ConstructorFn) {
	r.codecs[strings.ToLower(ext)] = fn
}

// NewFromFile creates and opens a decoder for the given audio file.
// The codec is selected by file extension. The decoder is returned in an opened state.
func (r *Registry) NewFromFile(fileName string, sampleBitDepth int) (AudioDecoder, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	fn, ok := r.codecs[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}

	dec, err := fn(sampleBitDepth)
	if err != nil {
		return nil, fmt.Errorf("creating decoder for %s: %w", ext, err)
	}

	if err := dec.Open(fileName); err != nil {
		return nil, fmt.Errorf("opening %s: %w", filepath.Base(fileName), err)
	}

	return dec, nil
}

// Supports returns true if the registry has a decoder for the given extension.
func (r *Registry) Supports(ext string) bool {
	_, ok := r.codecs[strings.ToLower(ext)]
	return ok
}

// Extensions returns all registered file extensions.
func (r *Registry) Extensions() []string {
	exts := make([]string, 0, len(r.codecs))
	for ext := range r.codecs {
		exts = append(exts, ext)
	}
	return exts
}
