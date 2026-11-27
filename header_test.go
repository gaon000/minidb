package minidb

import (
	"bytes"
	"testing"
)

func TestDatabaseHeaderLayout(t *testing.T) {
	header := DatabaseHeader{
		Magic:        [4]byte{'M', 'N', 'D', 'B'},
		Version:      1,
		RecordCount:  300,
		FreeListHead: 258,
		Reserved:     [8]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88},
	}
	wantBytes := []byte{
		'M', 'N', 'D', 'B',
		1, 0, 0, 0,
		0x2c, 0x01, 0, 0, 0, 0, 0, 0,
		0x02, 0x01, 0, 0, 0, 0, 0, 0,
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
	}
	if got := header.Serialize(); !bytes.Equal(got, wantBytes) {
		t.Fatalf("serialized header = % x, want % x", got, wantBytes)
	}

	decoded, err := DeserializeHeader(wantBytes)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != header {
		t.Fatalf("decoded header = %+v, want %+v", decoded, header)
	}
}

func TestEmptyFreeListEncoding(t *testing.T) {
	header := DatabaseHeader{Magic: [4]byte{'M', 'N', 'D', 'B'}, Version: 1, FreeListHead: -1}
	data := header.Serialize()
	wantHead := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	if got := data[16:24]; !bytes.Equal(got, wantHead) {
		t.Fatalf("encoded empty free-list head = % x, want % x", got, wantHead)
	}
	decoded, err := DeserializeHeader(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.FreeListHead != -1 {
		t.Fatalf("decoded empty free-list head = %d, want -1", decoded.FreeListHead)
	}
}

func TestDeserializeHeaderRejectsInvalidHeaders(t *testing.T) {
	valid := DatabaseHeader{Magic: [4]byte{'M', 'N', 'D', 'B'}, Version: 1, RecordCount: 3, FreeListHead: 1}
	badMagic, badVersion, badHead, negativeHead := valid, valid, valid, valid
	badMagic.Magic = [4]byte{}
	badVersion.Version = 2
	badHead.FreeListHead = 3
	negativeHead.FreeListHead = -2
	tests := []struct {
		label string
		data  []byte
	}{
		{label: "short header", data: valid.Serialize()[:31]},
		{label: "bad magic", data: badMagic.Serialize()},
		{label: "unsupported version", data: badVersion.Serialize()},
		{label: "head past last slot", data: badHead.Serialize()},
		{label: "invalid negative head", data: negativeHead.Serialize()},
	}
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			if _, err := DeserializeHeader(test.data); err == nil {
				t.Fatal("DeserializeHeader accepted an invalid header")
			}
		})
	}
}
