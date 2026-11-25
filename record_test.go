package minidb

import (
	"bytes"
	"testing"
)

func TestSerializeRecordLayout(t *testing.T) {
	got, err := Serialize(User{ID: 0x12345678, Name: "Alice", Age: 25})
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		0x78, 0x56, 0x34, 0x12,
		'A', 'l', 'i', 'c', 'e', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0x19,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("serialized record = % x, want % x", got, want)
	}
}

func TestSerializeNameAtByteLimit(t *testing.T) {
	const name = "가나다라마a" // 16 bytes in UTF-8.
	record, err := Serialize(User{Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(record[4:20]); got != name {
		t.Fatalf("encoded name = %q, want %q", got, name)
	}
}

func TestSerializeRejectsInvalidNames(t *testing.T) {
	tests := []struct {
		label string
		name  string
	}{
		{label: "over byte limit", name: "가나다라마바"},
		{label: "invalid UTF-8", name: "\xff"},
		{label: "NUL reserved for padding", name: "Bob\x00"},
	}
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			if _, err := Serialize(User{Name: test.name}); err == nil {
				t.Fatalf("Serialize accepted invalid name %q", test.name)
			}
		})
	}
}

func TestDeserializeRecordLayout(t *testing.T) {
	record := []byte{0x78, 0x56, 0x34, 0x12, 'B', 'o', 'b', 20: 0x19}
	got, err := Deserialize(record)
	if err != nil {
		t.Fatal(err)
	}
	want := User{ID: 0x12345678, Name: "Bob", Age: 25}
	if got != want {
		t.Fatalf("deserialized record = %+v, want %+v", got, want)
	}
}

func TestRecordRoundTripAtNameBoundaries(t *testing.T) {
	for _, name := range []string{"", "가나다라마a"} {
		want := User{ID: 42, Name: name, Age: 30}
		record, err := Serialize(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Deserialize(record)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("round-trip record = %+v, want %+v", got, want)
		}
	}
}

func TestDeserializeRejectsMalformedRecords(t *testing.T) {
	tests := []struct {
		label  string
		record []byte
	}{
		{label: "too short", record: make([]byte, 20)},
		{label: "too long", record: make([]byte, 22)},
		{label: "invalid UTF-8", record: []byte{4: 0xff, 20: 25}},
		{label: "embedded NUL", record: []byte{4: 'B', 6: 'b', 20: 25}},
	}
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			if _, err := Deserialize(test.record); err == nil {
				t.Fatal("Deserialize accepted a malformed record")
			}
		})
	}
}
