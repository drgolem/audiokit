package types

import "path"

// DocType classifies entries in the music document tree.
type DocType string

const (
	DocTypeSong       DocType = "song"
	DocTypeFolder     DocType = "folder"
	DocTypeCuesheet   DocType = "cuesheet"
	DocTypeMusicRoot  DocType = "musicroot"
	DocTypeMusicStore DocType = "musicstore"
)

// SongDocument represents a node in the music library tree.
type SongDocument struct {
	Type       DocType
	Song       *SongInfo `json:"Song,omitempty" bson:"Song,omitempty" structs:"Song,omitempty"`
	FolderName string    `db:"FolderName" json:"FolderName,omitempty" bson:"FolderName,omitempty" structs:"FolderName,omitempty"`
	Ancestors  []string  `db:"Ancestors" json:"Ancestors,omitempty" bson:"Ancestors,omitempty" structs:"Ancestors,omitempty"`
}

// FolderPath returns the full path for this document by joining ancestors and folder name.
func (sd SongDocument) FolderPath() string {
	if len(sd.Ancestors) == 0 {
		return sd.FolderName
	}
	parent := path.Join(sd.Ancestors...)
	return path.Join(parent, sd.FolderName)
}

// MusicDbDriverType identifies a database backend.
type MusicDbDriverType string

const (
	MusicDbMongo  MusicDbDriverType = "mongodb"
	MusicDbJson   MusicDbDriverType = "jsondb"
	MusicDbDuckDB MusicDbDriverType = "duckdb"
)
