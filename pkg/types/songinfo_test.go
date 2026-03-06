package types

import (
	"testing"
	"time"
)

func TestSongInfo_EnsureTitle(t *testing.T) {
	s := &SongInfo{FilePath: "/music/artist/album/01-track.flac"}
	s.EnsureTitle()
	if s.Title != "01-track.flac" {
		t.Errorf("EnsureTitle() = %q, want %q", s.Title, "01-track.flac")
	}

	// Should not overwrite existing title
	s2 := &SongInfo{Title: "Existing", FilePath: "/music/file.mp3"}
	s2.EnsureTitle()
	if s2.Title != "Existing" {
		t.Errorf("EnsureTitle() overwrote existing title: %q", s2.Title)
	}
}

func TestSongInfo_NormalizeMetadata(t *testing.T) {
	s := &SongInfo{FilePath: "artist/album/01-track.flac"}
	s.NormalizeMetadata()

	if s.Title != "01-track.flac" {
		t.Errorf("Title = %q, want %q", s.Title, "01-track.flac")
	}
	if s.Album != "album" {
		t.Errorf("Album = %q, want %q", s.Album, "album")
	}
	if s.Artist != "artist" {
		t.Errorf("Artist = %q, want %q", s.Artist, "artist")
	}
}

func TestSongInfo_NormalizeMetadata_NoOverwrite(t *testing.T) {
	s := &SongInfo{
		Title:    "My Song",
		Artist:   "My Artist",
		Album:    "My Album",
		FilePath: "other/path/file.mp3",
	}
	s.NormalizeMetadata()

	if s.Title != "My Song" {
		t.Errorf("Title overwritten: %q", s.Title)
	}
	if s.Artist != "My Artist" {
		t.Errorf("Artist overwritten: %q", s.Artist)
	}
	if s.Album != "My Album" {
		t.Errorf("Album overwritten: %q", s.Album)
	}
}

func TestGenerateSongID(t *testing.T) {
	id1 := GenerateSongID("music/song.flac", 0)
	id2 := GenerateSongID("music/song.flac", 0)
	id3 := GenerateSongID("music/song.flac", 30*time.Second)

	if id1 != id2 {
		t.Error("Same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Error("Different startPos should produce different ID")
	}
	if id1 == "" {
		t.Error("ID should not be empty")
	}
}

func TestResolveFilePath(t *testing.T) {
	tests := []struct {
		root, rel, want string
	}{
		{"/music", "artist/song.mp3", "/music/artist/song.mp3"},
		{"/music", "/absolute/path.flac", "/absolute/path.flac"}, // absolute = legacy
		{"", "song.mp3", "song.mp3"},                             // no root
		{"/music", "", ""},                                       // no path
	}
	for _, tt := range tests {
		got := ResolveFilePath(tt.root, tt.rel)
		if got != tt.want {
			t.Errorf("ResolveFilePath(%q, %q) = %q, want %q", tt.root, tt.rel, got, tt.want)
		}
	}
}

func TestToRelativePath(t *testing.T) {
	tests := []struct {
		root, abs, want string
	}{
		{"/music", "/music/artist/song.mp3", "artist/song.mp3"},
		{"/music", "already/relative.mp3", "already/relative.mp3"},
		{"/music", "/other/path.flac", "/other/path.flac"}, // not under root
		{"", "/music/song.mp3", "/music/song.mp3"},         // no root
	}
	for _, tt := range tests {
		got := ToRelativePath(tt.root, tt.abs)
		if got != tt.want {
			t.Errorf("ToRelativePath(%q, %q) = %q, want %q", tt.root, tt.abs, got, tt.want)
		}
	}
}
