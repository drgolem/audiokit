package oggutil

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"os"

	"github.com/drgolem/go-ogg/ogg"
)

// Stream type detection patterns.
var (
	VorbisPattern   = [6]byte{'v', 'o', 'r', 'b', 'i', 's'}
	OpusHeadPattern = [8]byte{'O', 'p', 'u', 's', 'H', 'e', 'a', 'd'}
	OpusTagsPattern = [8]byte{'O', 'p', 'u', 's', 'T', 'a', 'g', 's'}
)

// StreamType identifies the codec inside an OGG container.
type StreamType int

const (
	StreamTypeUnknown StreamType = iota
	StreamTypeVorbis
	StreamTypeOpus
)

// OggReader is the interface for reading OGG page streams.
type OggReader interface {
	Next() bool
	Scan() ([]byte, error)
	Close()
}

// VorbisCommonHeader is the common prefix of all Vorbis packets.
type VorbisCommonHeader struct {
	PacketType    byte
	VorbisPattern [6]byte
}

// VorbisIdentificationHeader follows VorbisCommonHeader in packet type 1.
type VorbisIdentificationHeader struct {
	Version         uint32
	AudioChannels   byte
	AudioSampleRate uint32
	BitrateMax      int32
	BitrateMin      int32
	BlockSize01     uint32
	FramingFlag     byte
}

// OpusCommonHeader is the prefix of Opus identification packets.
type OpusCommonHeader struct {
	OpusPattern [8]byte
}

// OpusIdentificationHeader follows OpusCommonHeader.
type OpusIdentificationHeader struct {
	Version         byte
	AudioChannels   byte
	PreSkip         uint16
	AudioSampleRate uint32
	OutputGain      uint16
	MappingFamily   byte
}

// GetOggFileStreamType detects whether an OGG file contains Vorbis or Opus audio.
func GetOggFileStreamType(fileName string) (StreamType, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return StreamTypeUnknown, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	oggReader, err := ogg.NewOggReader(reader)
	if err != nil {
		return StreamTypeUnknown, err
	}

	if oggReader.Next() {
		p, err := oggReader.Scan()
		if err != nil {
			return StreamTypeUnknown, err
		}

		// Try Vorbis
		bytesReader := bytes.NewReader(p)
		var ch VorbisCommonHeader
		if err := binary.Read(bytesReader, binary.LittleEndian, &ch); err == nil {
			if ch.PacketType == 1 && ch.VorbisPattern == VorbisPattern {
				return StreamTypeVorbis, nil
			}
		}

		// Try Opus
		bytesReader = bytes.NewReader(p)
		var coh OpusCommonHeader
		if err := binary.Read(bytesReader, binary.LittleEndian, &coh); err == nil {
			if coh.OpusPattern == OpusHeadPattern {
				return StreamTypeOpus, nil
			}
		}
	}

	return StreamTypeUnknown, nil
}
