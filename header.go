package minidb

import (
	"encoding/binary"
	"fmt"
)

const (
	// HeaderSize is the size in bytes of an encoded database header.
	HeaderSize = 32

	headerMagic   = "MNDB"
	headerVersion = 1
	noFreeSlot    = -1
)

// DatabaseHeader describes the file format and tracks allocated record slots.
// FreeListHead is -1 when there are no reusable slots.
type DatabaseHeader struct {
	Magic        [4]byte
	Version      uint32
	RecordCount  uint64
	FreeListHead int64
	Reserved     [8]byte
}

// Serialize encodes the header into its fixed 32-byte file layout.
func (h DatabaseHeader) Serialize() []byte {
	data := make([]byte, HeaderSize)
	copy(data[0:4], h.Magic[:])
	binary.LittleEndian.PutUint32(data[4:8], h.Version)
	binary.LittleEndian.PutUint64(data[8:16], h.RecordCount)
	binary.LittleEndian.PutUint64(data[16:24], uint64(h.FreeListHead))
	copy(data[24:32], h.Reserved[:])
	return data
}

// DeserializeHeader reads a database header and checks its format and free-list head.
func DeserializeHeader(data []byte) (DatabaseHeader, error) {
	if len(data) != HeaderSize {
		return DatabaseHeader{}, fmt.Errorf("database header is %d bytes, want %d", len(data), HeaderSize)
	}

	header := DatabaseHeader{
		Magic:        [4]byte(data[0:4]),
		Version:      binary.LittleEndian.Uint32(data[4:8]),
		RecordCount:  binary.LittleEndian.Uint64(data[8:16]),
		FreeListHead: int64(binary.LittleEndian.Uint64(data[16:24])),
		Reserved:     [8]byte(data[24:32]),
	}
	if string(header.Magic[:]) != headerMagic {
		return DatabaseHeader{}, fmt.Errorf("database header magic is %q, want %q", header.Magic, headerMagic)
	}
	if header.Version != headerVersion {
		return DatabaseHeader{}, fmt.Errorf("unsupported database format version %d", header.Version)
	}
	if header.FreeListHead != noFreeSlot && (header.FreeListHead < 0 || uint64(header.FreeListHead) >= header.RecordCount) {
		return DatabaseHeader{}, fmt.Errorf("free-list head %d is out of bounds for %d slots", header.FreeListHead, header.RecordCount)
	}
	return header, nil
}
