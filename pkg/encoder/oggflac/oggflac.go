package oggflac

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
)

// Writer wraps FLAC frames in Ogg pages per the Ogg FLAC mapping spec.
// See: https://xiph.org/flac/ogg_mapping.html
type Writer struct {
	w         io.Writer
	serialNo  uint32
	granule   uint64
	pageSeqNo uint32
}

// NewWriter creates a new Ogg FLAC writer and writes the BOS and
// header pages. streamInfo must be a 34-byte STREAMINFO block.
func NewWriter(w io.Writer, streamInfo []byte, sampleRate, channels, bps int) (*Writer, error) {
	if len(streamInfo) != 34 {
		return nil, fmt.Errorf("STREAMINFO must be 34 bytes, got %d", len(streamInfo))
	}

	ow := &Writer{
		w:        w,
		serialNo: rand.Uint32(),
	}

	// Write BOS page: Ogg FLAC mapping header
	if err := ow.writeBOS(streamInfo); err != nil {
		return nil, fmt.Errorf("write BOS page: %w", err)
	}

	// Write VORBIS_COMMENT header page (required by Ogg FLAC spec)
	if err := ow.writeVorbisComment(); err != nil {
		return nil, fmt.Errorf("write VORBIS_COMMENT page: %w", err)
	}

	return ow, nil
}

// WriteFrame wraps a FLAC audio frame in an Ogg page and writes it.
// samplesInFrame is the number of samples (per channel) in this frame.
func (ow *Writer) WriteFrame(flacFrame []byte, samplesInFrame int) error {
	if len(flacFrame) == 0 {
		return nil
	}

	ow.granule += uint64(samplesInFrame)

	return ow.writePage(flacFrame, 0x00, ow.granule) // continuation=0, no BOS/EOS
}

// Close writes the EOS page and flushes.
func (ow *Writer) Close() error {
	return ow.writePage(nil, 0x04, ow.granule) // headerType 0x04 = EOS
}

// writeBOS writes the Beginning of Stream page with the Ogg FLAC mapping header.
func (ow *Writer) writeBOS(streamInfo []byte) error {
	// Total BOS packet: 9 (mapping header) + 4 (fLaC) + 4 (block header) + 34 (STREAMINFO) = 51 bytes
	packet := make([]byte, 51)

	// Ogg FLAC mapping header
	packet[0] = 0x7F                                   // packet type
	copy(packet[1:5], "FLAC")                          // magic
	packet[5] = 1                                      // major version
	packet[6] = 0                                      // minor version
	binary.BigEndian.PutUint16(packet[7:9], 1)         // 1 non-audio header packet (VORBIS_COMMENT)

	// Native FLAC stream marker
	copy(packet[9:13], "fLaC")

	// STREAMINFO metadata block header: type=0 (STREAMINFO), not last (0x00), length=34
	packet[13] = 0x00 // type=0, not last
	packet[14] = 0x00
	packet[15] = 0x00
	packet[16] = 34 // length of STREAMINFO data

	// STREAMINFO data
	copy(packet[17:51], streamInfo)

	return ow.writePage(packet, 0x02, 0) // headerType 0x02 = BOS
}

// writeVorbisComment writes a minimal VORBIS_COMMENT metadata block.
// This is required by the Ogg FLAC mapping spec.
func (ow *Writer) writeVorbisComment() error {
	vendor := "audiokit"
	// Block header (4) + vendor length (4) + vendor string + comment count (4)
	dataLen := 4 + len(vendor) + 4
	packet := make([]byte, 4+dataLen)

	// FLAC metadata block header: type=4 (VORBIS_COMMENT), is_last=1
	packet[0] = 0x84 // 1<<7 | 4 (last block flag + type 4)
	// Length as 24-bit BE
	packet[1] = byte(dataLen >> 16)
	packet[2] = byte(dataLen >> 8)
	packet[3] = byte(dataLen)

	// Vendor string (little-endian length + UTF-8 string)
	binary.LittleEndian.PutUint32(packet[4:8], uint32(len(vendor)))
	copy(packet[8:8+len(vendor)], vendor)

	// User comment count = 0
	binary.LittleEndian.PutUint32(packet[8+len(vendor):], 0)

	return ow.writePage(packet, 0x00, 0) // granule 0 for header pages
}

// writePage writes a single Ogg page containing the given payload.
func (ow *Writer) writePage(payload []byte, headerType byte, granulePos uint64) error {
	segments := segmentTable(payload)

	// Page header: 27 bytes + segment table
	headerSize := 27 + len(segments)
	page := make([]byte, headerSize+len(payload))

	// Capture pattern
	copy(page[0:4], "OggS")
	page[4] = 0                                              // stream structure version
	page[5] = headerType                                     // header type flags
	binary.LittleEndian.PutUint64(page[6:14], granulePos)    // granule position
	binary.LittleEndian.PutUint32(page[14:18], ow.serialNo)  // stream serial number
	binary.LittleEndian.PutUint32(page[18:22], ow.pageSeqNo) // page sequence number
	// page[22:26] = CRC (filled below)
	page[26] = byte(len(segments)) // number of segments

	// Segment table
	copy(page[27:27+len(segments)], segments)

	// Payload
	copy(page[headerSize:], payload)

	// CRC32 (with checksum bytes zeroed during calculation)
	binary.LittleEndian.PutUint32(page[22:26], 0)
	crc := oggCRC32(page)
	binary.LittleEndian.PutUint32(page[22:26], crc)

	ow.pageSeqNo++

	_, err := ow.w.Write(page)
	return err
}

// segmentTable builds the Ogg segment table for a payload.
func segmentTable(payload []byte) []byte {
	if len(payload) == 0 {
		return []byte{0}
	}

	n := len(payload)
	numFull := n / 255
	remainder := n % 255

	segments := make([]byte, 0, numFull+1)
	for i := 0; i < numFull; i++ {
		segments = append(segments, 255)
	}
	segments = append(segments, byte(remainder))

	return segments
}

// Ogg CRC32 lookup table using polynomial 0x04C11DB7.
var oggCRCTable [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if r&0x80000000 != 0 {
				r = (r << 1) ^ 0x04C11DB7
			} else {
				r <<= 1
			}
		}
		oggCRCTable[i] = r
	}
}

// oggCRC32 computes the Ogg-specific CRC32 of data.
func oggCRC32(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc = (crc << 8) ^ oggCRCTable[byte(crc>>24)^b]
	}
	return crc
}
