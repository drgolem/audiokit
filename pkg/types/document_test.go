package types

import "testing"

func TestSongDocument_FolderPath(t *testing.T) {
	tests := []struct {
		name     string
		doc      SongDocument
		want     string
	}{
		{
			"no ancestors",
			SongDocument{FolderName: "album"},
			"album",
		},
		{
			"with ancestors",
			SongDocument{FolderName: "album", Ancestors: []string{"music", "artist"}},
			"music/artist/album",
		},
		{
			"empty",
			SongDocument{},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.doc.FolderPath()
			if got != tt.want {
				t.Errorf("FolderPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
