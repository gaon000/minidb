package minidb

import (
	"errors"
	"fmt"
	"os"
)

// ErrRecordNotFound indicates that a RID is outside the stored records.
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
