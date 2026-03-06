package types

import (
	"fmt"
	"hash/fnv"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SongInfo holds metadata for a single audio track.
type SongInfo struct {
	Title      string
	Artist     string
	Album      string
	Genre      string
	Duration   time.Duration
	StartPos   time.Duration
	Format     FrameFormat
	FileFormat FileFormatType
	// FilePath is the path to the audio file, relative to the music root directory.
	// May be absolute for legacy compatibility (use ResolveFilePath to get absolute path).
	FilePath string `json:"FilePath,omitempty" bson:"FilePath,omitempty" structs:"FilePath,omitempty"`
	ID       string `json:"ID,omitempty" bson:"ID,omitempty" structs:"ID,omitempty"`

	// ReplayGain metadata — two independent sources, both can coexist.
	// nil pointer = no data from that source.
	RGEmbedded *ReplayGainValues `json:"rgEmbedded,omitempty" bson:"rgEmbedded,omitempty"`
	RGComputed *ReplayGainValues `json:"rgComputed,omitempty" bson:"rgComputed,omitempty"`
}

// ReplayGainValues holds gain/peak data from a single source.
type ReplayGainValues struct {
	TrackGain float64 `json:"trackGain,omitempty"`
	TrackPeak float64 `json:"trackPeak,omitempty"`
	AlbumGain float64 `json:"albumGain,omitempty"`
	AlbumPeak float64 `json:"albumPeak,omitempty"`
}

func (s *SongInfo) String() string {
	return fmt.Sprintf("Duration: [%s], Title: [%s], Artist: [%s], Album: [%s], Format: [%s], file: [%s]",
		s.Duration, s.Title, s.Artist, s.Album, s.Format.String(), s.FilePath)
}

// EnsureTitle sets the title to the filename if it is empty.
func (s *SongInfo) EnsureTitle() {
	if s.Title == "" && s.FilePath != "" {
		s.Title = path.Base(s.FilePath)
	}
}

// NormalizeMetadata fills empty metadata fields from the file path:
//   - Title: filename
//   - Album: parent directory name
//   - Artist: grandparent directory name
func (s *SongInfo) NormalizeMetadata() {
	if s.FilePath == "" {
		return
	}

	if s.Title == "" {
		s.Title = path.Base(s.FilePath)
	}

	dirPath := path.Dir(s.FilePath)

	if s.Album == "" && dirPath != "" && dirPath != "." {
		s.Album = path.Base(dirPath)
	}

	if s.Artist == "" && dirPath != "" && dirPath != "." {
		parentDir := path.Dir(dirPath)
		if parentDir != "" && parentDir != "." && parentDir != "/" {
			s.Artist = path.Base(parentDir)
		} else {
			s.Artist = "Unknown Artist"
		}
	}
}

// GenerateSongID produces a stable, deterministic ID for a song based on its
// file path and start position. The ID is a base36 string from FNV-64a hash.
// StartPos distinguishes CUE tracks that share the same audio file.
func GenerateSongID(filePath string, startPos time.Duration) string {
	h := fnv.New64a()
	h.Write([]byte(filePath))
	h.Write([]byte{0})
	h.Write([]byte(strconv.FormatInt(startPos.Nanoseconds(), 10)))
	return strconv.FormatUint(h.Sum64(), 36)
}

// ResolveFilePath returns the absolute path for a file.
// If relPath is already absolute (legacy data), it is returned as-is.
func ResolveFilePath(musicRoot, relPath string) string {
	if relPath == "" || musicRoot == "" {
		return relPath
	}
	if filepath.IsAbs(relPath) {
		return relPath
	}
	return filepath.Join(musicRoot, relPath)
}

// ToRelativePath strips musicRoot prefix from an absolute path.
// Returns absPath as-is if it is not under musicRoot or is already relative.
func ToRelativePath(musicRoot, absPath string) string {
	if absPath == "" || musicRoot == "" {
		return absPath
	}
	if !filepath.IsAbs(absPath) {
		return absPath
	}
	rel, err := filepath.Rel(musicRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return rel
}
