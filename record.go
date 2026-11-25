package minidb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	nameOffset = 4
	nameSize   = 16
	ageOffset  = nameOffset + nameSize

	// RecordSize is the size in bytes of an encoded user record.
	RecordSize = ageOffset + 1
)

// User is the fixed schema for the first storage exercise.
type User struct {
	ID   uint32
	Name string
	Age  uint8
}

// Serialize encodes the ID in little-endian order, followed by the
// zero-padded UTF-8 name and the single-byte age.
// Names can occupy up to 16 bytes and cannot contain NUL, which is reserved for padding.
func Serialize(user User) ([]byte, error) {
	if err := validateName(user.Name); err != nil {
		return nil, err
	}

	record := make([]byte, RecordSize)
	binary.LittleEndian.PutUint32(record[:nameOffset], user.ID)
	copy(record[nameOffset:ageOffset], user.Name)
	record[ageOffset] = user.Age
	return record, nil
}

// Deserialize decodes a fixed-size user record and removes trailing name padding.
func Deserialize(record []byte) (User, error) {
	if len(record) != RecordSize {
		return User{}, fmt.Errorf("record is %d bytes, want %d", len(record), RecordSize)
	}

	name := strings.TrimRight(string(record[nameOffset:ageOffset]), "\x00")
	if err := validateName(name); err != nil {
		return User{}, err
	}
	return User{
		ID:   binary.LittleEndian.Uint32(record[:nameOffset]),
		Name: name,
		Age:  record[ageOffset],
	}, nil
}

func validateName(name string) error {
	if len(name) > nameSize {
		return fmt.Errorf("name is %d bytes, maximum is %d", len(name), nameSize)
	}
	if !utf8.ValidString(name) || strings.IndexByte(name, 0) >= 0 {
		return errors.New("name must be valid UTF-8 without NUL bytes")
	}
	return nil
}
