package types

// PlaylistItem is a song with its position in a playlist.
type PlaylistItem struct {
	PlaylistID int
	SongInfo
}

// PlaylistCollection maps playlist names to their items.
type PlaylistCollection map[string][]PlaylistItem

// BookPosition stores the last known playback position for an audiobook/playlist.
type BookPosition struct {
	Book                string `json:"book"`
	ChapterID           int    `json:"chapterId"`
	ElapsedNs           int64  `json:"elapsed"`
	UpdatedAt           int64  `json:"updatedAt"`
	Completed           bool   `json:"completed"`
	CompletedChapters   []int  `json:"completedChapters,omitempty"`
	UncompletedChapters []int  `json:"uncompletedChapters,omitempty"`
}

// Attrs is a generic string key-value map used for status reporting.
type Attrs map[string]string
