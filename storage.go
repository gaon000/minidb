package minidb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
)

const (
	slotSize          = 1 + RecordSize
	slotInUse    byte = 0x00
	slotDeleted  byte = 0x01
	maxSlotCount      = (math.MaxInt64 - HeaderSize) / slotSize
)

// ErrRecordNotFound indicates that a RID has no live record.
var ErrRecordNotFound = errors.New("record not found")

// Storage holds fixed-size user records in a single file. Open the file once
// and call its methods sequentially.
type Storage struct {
	file        *os.File
	recordCount int64
}

// Open opens or creates a file containing consecutive user records.
func Open(path string) (*Storage, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if !info.Mode().IsRegular() || info.Size()%RecordSize != 0 {
		err := fmt.Errorf("storage must be a regular file containing whole %d-byte records", RecordSize)
		return nil, errors.Join(err, file.Close())
	}
	return &Storage{file: file, recordCount: info.Size() / RecordSize}, nil
}

// OpenDatabaseFile opens a file using the lesson 2 header format. A new or
// empty file receives an initial header. The caller must close the returned file.
// The returned header is a snapshot of the file at open time.
func OpenDatabaseFile(path string) (_ *os.File, _ DatabaseHeader, err error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, DatabaseHeader{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, file.Close())
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return nil, DatabaseHeader{}, err
	}
	if !info.Mode().IsRegular() {
		return nil, DatabaseHeader{}, errors.New("database must be a regular file")
	}

	if info.Size() == 0 {
		header := DatabaseHeader{Version: headerVersion, FreeListHead: noFreeSlot}
		copy(header.Magic[:], headerMagic)
		written, err := file.WriteAt(header.Serialize(), 0)
		if err != nil {
			if written > 0 {
				err = errors.Join(err, file.Truncate(0))
			}
			return nil, DatabaseHeader{}, fmt.Errorf("write initial database header: %w", err)
		}
		return file, header, nil
	}

	header, err := readDatabaseHeader(file)
	if err != nil {
		return nil, DatabaseHeader{}, err
	}
	return file, header, nil
}

func readDatabaseHeader(file *os.File) (DatabaseHeader, error) {
	var data [HeaderSize]byte
	if _, err := file.ReadAt(data[:], 0); err != nil {
		return DatabaseHeader{}, fmt.Errorf("read database header: %w", err)
	}
	return DeserializeHeader(data[:])
}

// InsertRecord inserts a lesson 2 record and returns its zero-based RID.
// Call it sequentially on the file returned by OpenDatabaseFile.
func InsertRecord(file *os.File, user User) (int64, error) {
	payload, err := Serialize(user)
	if err != nil {
		return 0, err
	}
	header, err := readDatabaseHeader(file)
	if err != nil {
		return 0, err
	}
	if header.RecordCount > maxSlotCount {
		return 0, fmt.Errorf("database slot count is too large: %d", header.RecordCount)
	}
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if expected := HeaderSize + int64(header.RecordCount)*slotSize; info.Size() != expected {
		return 0, fmt.Errorf("database file is %d bytes, want %d for %d slots", info.Size(), expected, header.RecordCount)
	}

	updatedHeader := header
	rid := header.FreeListHead
	var oldSlot []byte
	if rid == noFreeSlot {
		if header.RecordCount == maxSlotCount {
			return 0, errors.New("database cannot allocate another slot")
		}
		rid = int64(header.RecordCount)
		updatedHeader.RecordCount++
	} else {
		oldSlot = make([]byte, slotSize)
		if _, err := file.ReadAt(oldSlot, HeaderSize+rid*slotSize); err != nil {
			return 0, fmt.Errorf("read free slot %d: %w", rid, err)
		}
		if oldSlot[0] != slotDeleted {
			return 0, fmt.Errorf("free-list head points to slot %d with status %#02x", rid, oldSlot[0])
		}
		next := int64(binary.LittleEndian.Uint64(oldSlot[1:9]))
		if next != noFreeSlot && (next < 0 || uint64(next) >= header.RecordCount || next == rid) {
			return 0, fmt.Errorf("free slot %d has invalid next RID %d", rid, next)
		}
		updatedHeader.FreeListHead = next
	}

	var slot [slotSize]byte
	slot[0] = slotInUse
	copy(slot[1:], payload)
	if err := writeSlotAndHeader(file, rid, slot[:], oldSlot, updatedHeader, header); err != nil {
		return 0, err
	}
	return rid, nil
}

// GetRecord reads the live lesson 2 record at rid.
// A deleted or out-of-range RID returns ErrRecordNotFound.
func GetRecord(file *os.File, rid int64) (User, error) {
	header, err := readDatabaseHeader(file)
	if err != nil {
		return User{}, err
	}
	slot, err := readOccupiedSlot(file, header, rid)
	if err != nil {
		return User{}, err
	}
	return Deserialize(slot[1:])
}

// DeleteRecord adds an occupied lesson 2 slot to the front of the free list.
// Call it sequentially on the file returned by OpenDatabaseFile.
func DeleteRecord(file *os.File, rid int64) error {
	header, err := readDatabaseHeader(file)
	if err != nil {
		return err
	}
	oldSlot, err := readOccupiedSlot(file, header, rid)
	if err != nil {
		return err
	}
	if header.FreeListHead == rid {
		return fmt.Errorf("free-list head points to occupied slot %d", rid)
	}

	var freeSlot [slotSize]byte
	freeSlot[0] = slotDeleted
	binary.LittleEndian.PutUint64(freeSlot[1:9], uint64(header.FreeListHead))
	updatedHeader := header
	updatedHeader.FreeListHead = rid
	return writeSlotAndHeader(file, rid, freeSlot[:], oldSlot[:], updatedHeader, header)
}

func readOccupiedSlot(file *os.File, header DatabaseHeader, rid int64) ([slotSize]byte, error) {
	var slot [slotSize]byte
	if header.RecordCount > maxSlotCount {
		return slot, fmt.Errorf("database slot count is too large: %d", header.RecordCount)
	}
	if rid < 0 || uint64(rid) >= header.RecordCount {
		return slot, fmt.Errorf("RID %d: %w", rid, ErrRecordNotFound)
	}
	if _, err := file.ReadAt(slot[:], HeaderSize+rid*slotSize); err != nil {
		return slot, fmt.Errorf("read slot %d: %w", rid, err)
	}
	switch slot[0] {
	case slotInUse:
		return slot, nil
	case slotDeleted:
		return slot, fmt.Errorf("RID %d: %w", rid, ErrRecordNotFound)
	default:
		return slot, fmt.Errorf("slot %d has invalid status %#02x", rid, slot[0])
	}
}

// writeSlotAndHeader writes the slot before publishing the updated header.
// A nil oldSlot indicates an append at the previous end of the file.
func writeSlotAndHeader(file *os.File, rid int64, slot, oldSlot []byte, header, oldHeader DatabaseHeader) error {
	offset := HeaderSize + rid*slotSize
	written, err := file.WriteAt(slot, offset)
	if err != nil {
		err = fmt.Errorf("write slot %d: %w", rid, err)
		if written > 0 {
			err = errors.Join(err, restoreSlot(file, offset, oldSlot))
		}
		return err
	}

	written, err = file.WriteAt(header.Serialize(), 0)
	if err != nil {
		err = fmt.Errorf("write database header: %w", err)
		if written > 0 {
			if _, restoreErr := file.WriteAt(oldHeader.Serialize(), 0); restoreErr != nil {
				err = errors.Join(err, fmt.Errorf("restore database header: %w", restoreErr))
			}
		}
		return errors.Join(err, restoreSlot(file, offset, oldSlot))
	}
	return nil
}

func restoreSlot(file *os.File, offset int64, oldSlot []byte) error {
	var err error
	if oldSlot == nil {
		err = file.Truncate(offset)
	} else {
		_, err = file.WriteAt(oldSlot, offset)
	}
	if err != nil {
		return fmt.Errorf("restore slot at offset %d: %w", offset, err)
	}
	return nil
}

// Insert appends a user and returns its zero-based record ID.
func (s *Storage) Insert(user User) (int64, error) {
	record, err := Serialize(user)
	if err != nil {
		return 0, err
	}

	rid := s.recordCount
	offset := rid * RecordSize
	written, err := s.file.WriteAt(record, offset)
	if err != nil {
		if written > 0 {
			err = errors.Join(err, s.file.Truncate(offset))
		}
		return 0, fmt.Errorf("write record %d: %w", rid, err)
	}
	s.recordCount++
	return rid, nil
}

// Get reads a user by its zero-based record ID.
func (s *Storage) Get(rid int64) (User, error) {
	if rid < 0 || rid >= s.recordCount {
		return User{}, fmt.Errorf("RID %d: %w", rid, ErrRecordNotFound)
	}

	var record [RecordSize]byte
	if _, err := s.file.ReadAt(record[:], rid*RecordSize); err != nil {
		return User{}, fmt.Errorf("read record %d: %w", rid, err)
	}
	return Deserialize(record[:])
}

// Close releases the storage file.
func (s *Storage) Close() error {
	return s.file.Close()
}
