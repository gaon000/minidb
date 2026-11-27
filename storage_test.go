package minidb

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestStoragePersistsRecordsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.db")
	storage, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	users := []User{
		{ID: 101, Name: "Alice", Age: 25},
		{ID: 42, Name: "Bob", Age: 30},
		{ID: 98, Name: "Charlie", Age: 28},
	}
	for index, user := range users {
		rid, err := storage.Insert(user)
		if err != nil {
			t.Fatal(err)
		}
		if rid != int64(index) {
			t.Fatalf("RID = %d, want %d", rid, index)
		}
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(len(users) * RecordSize); info.Size() != want {
		t.Fatalf("file size = %d bytes, want %d", info.Size(), want)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	for _, rid := range []int64{2, 0, 1} {
		got, err := reopened.Get(rid)
		if err != nil {
			t.Fatal(err)
		}
		if want := users[rid]; got != want {
			t.Fatalf("Get(%d) = %+v, want %+v", rid, got, want)
		}
	}

	dana := User{ID: 81, Name: "Dana", Age: 34}
	rid, err := reopened.Insert(dana)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(len(users)); rid != want {
		t.Fatalf("RID after reopen = %d, want %d", rid, want)
	}
	got, err := reopened.Get(rid)
	if err != nil {
		t.Fatal(err)
	}
	if got != dana {
		t.Fatalf("Get(%d) = %+v, want %+v", rid, got, dana)
	}
}

func TestStorageRejectsMissingRIDs(t *testing.T) {
	storage, err := Open(filepath.Join(t.TempDir(), "users.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	for _, rid := range []int64{-1, 0, math.MaxInt64} {
		if _, err := storage.Get(rid); !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("Get(%d) error = %v, want ErrRecordNotFound", rid, err)
		}
	}
}

func TestOpenRejectsPartialRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.db")
	if err := os.WriteFile(path, make([]byte, RecordSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if storage, err := Open(path); err == nil {
		_ = storage.Close()
		t.Fatal("Open accepted a file ending in a partial record")
	}
}

func TestOpenDatabaseFileInitializesHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.db")
	file, header, err := OpenDatabaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	want := DatabaseHeader{Magic: [4]byte{'M', 'N', 'D', 'B'}, Version: 1, FreeListHead: -1}
	if header != want {
		t.Fatalf("new header = %+v, want %+v", header, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want.Serialize()) {
		t.Fatalf("new file = % x, want % x", data, want.Serialize())
	}
}

func TestOpenDatabaseFileLoadsExistingHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.db")
	want := DatabaseHeader{Magic: [4]byte{'M', 'N', 'D', 'B'}, Version: 1, RecordCount: 1, FreeListHead: 0}
	// One deleted slot with -1 as the next free RID.
	slot := []byte{0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 21: 0}
	contents := append(want.Serialize(), slot...)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	file, header, err := OpenDatabaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if header != want {
		t.Fatalf("existing header = %+v, want %+v", header, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, contents) {
		t.Fatal("opening changed the existing file")
	}
}

func TestOpenDatabaseFileRejectsInvalidHeadersWithoutReplacingThem(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "truncated header", data: []byte("MNDB")},
		{name: "lesson 1 data", data: make([]byte, 2*RecordSize)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "users.db")
			if err := os.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if file, _, err := OpenDatabaseFile(path); err == nil {
				_ = file.Close()
				t.Fatal("opened a file with an invalid database header")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, test.data) {
				t.Fatal("opening changed the invalid file")
			}
		})
	}
}

func TestDeleteRecordLinksDeletedSlots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.db")
	header := DatabaseHeader{Magic: [4]byte{'M', 'N', 'D', 'B'}, Version: 1, RecordCount: 3, FreeListHead: -1}
	contents := header.Serialize()
	for _, name := range []string{"Alice", "Bob", "Charlie"} {
		payload, err := Serialize(User{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		contents = append(contents, 0x00)
		contents = append(contents, payload...)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	file, _, err := OpenDatabaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	for _, rid := range []int64{1, 0} {
		if err := DeleteRecord(file, rid); err != nil {
			t.Fatal(err)
		}
	}
	for _, rid := range []int64{0, 1, -1, 3} {
		if err := DeleteRecord(file, rid); !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("DeleteRecord(%d) error = %v, want ErrRecordNotFound", rid, err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header.FreeListHead = 0
	want := header.Serialize()
	want = append(want, []byte{0x01, 0x01, 21: 0}...)
	want = append(want, []byte{0x01, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 21: 0}...)
	want = append(want, contents[HeaderSize+2*22:]...)
	if !bytes.Equal(got, want) {
		t.Fatalf("database after deletion = % x, want % x", got, want)
	}
}

func TestInsertRecordReusesFreeSlotsBeforeAppending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.db")
	file, _, err := OpenDatabaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	for index, name := range []string{"Alice", "Bob", "Charlie"} {
		rid, err := InsertRecord(file, User{Name: name})
		if err != nil || rid != int64(index) {
			t.Fatalf("insert %s = RID %d, %v, want RID %d", name, rid, err, index)
		}
	}
	for _, rid := range []int64{1, 0} {
		if err := DeleteRecord(file, rid); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, _, err := OpenDatabaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if _, err := InsertRecord(reopened, User{Name: "name longer than sixteen bytes"}); err == nil {
		t.Fatal("InsertRecord accepted an oversized name")
	}
	steps := []struct {
		name     string
		wantRID  int64
		wantHead int64
		wantSize int64
	}{
		{name: "Dana", wantRID: 0, wantHead: 1, wantSize: 98},
		{name: "Frank", wantRID: 1, wantHead: -1, wantSize: 98},
		{name: "Grace", wantRID: 3, wantHead: -1, wantSize: 120},
	}
	for _, step := range steps {
		rid, err := InsertRecord(reopened, User{Name: step.name})
		if err != nil || rid != step.wantRID {
			t.Fatalf("insert %s = RID %d, %v, want RID %d", step.name, rid, err, step.wantRID)
		}
		header, err := readDatabaseHeader(reopened)
		if err != nil {
			t.Fatal(err)
		}
		wantCount := uint64((step.wantSize - 32) / 22)
		if header.FreeListHead != step.wantHead || header.RecordCount != wantCount {
			t.Fatalf("after inserting %s, head and count = %d, %d, want %d, %d", step.name, header.FreeListHead, header.RecordCount, step.wantHead, wantCount)
		}
		info, err := reopened.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() != step.wantSize {
			t.Fatalf("after inserting %s, file size = %d bytes, want %d", step.name, info.Size(), step.wantSize)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for rid, name := range []string{"Dana", "Frank", "Charlie", "Grace"} {
		offset := 32 + rid*22
		if data[offset] != 0x00 {
			t.Fatalf("RID %d is not marked in use", rid)
		}
		user, err := Deserialize(data[offset+1 : offset+22])
		if err != nil || user.Name != name {
			t.Fatalf("RID %d = %+v, %v, want %q", rid, user, err, name)
		}
	}
}

func TestInsertRecordRejectsInvalidFreeListWithoutChangingFile(t *testing.T) {
	header := DatabaseHeader{Magic: [4]byte{'M', 'N', 'D', 'B'}, Version: 1, RecordCount: 1, FreeListHead: 0}
	tests := []struct {
		name string
		slot []byte
	}{
		{name: "truncated slot", slot: []byte{0x01}},
		{name: "occupied head", slot: make([]byte, 22)},
		{name: "self-referencing next RID", slot: []byte{0x01, 21: 0}},
		{name: "out-of-range next RID", slot: []byte{0x01, 0x01, 21: 0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "users.db")
			contents := append(header.Serialize(), test.slot...)
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			file, _, err := OpenDatabaseFile(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			if _, err := InsertRecord(file, User{Name: "Dana"}); err == nil {
				t.Fatal("InsertRecord accepted an invalid database")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, contents) {
				t.Fatal("InsertRecord changed the invalid database")
			}
		})
	}
}

func TestGetRecordAfterDeleteAndReuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.db")
	file, _, err := OpenDatabaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := InsertRecord(file, User{ID: 101, Name: "Alice", Age: 25}); err != nil {
		t.Fatal(err)
	}
	bob := User{ID: 42, Name: "Bob", Age: 30}
	rid, err := InsertRecord(file, bob)
	if err != nil || rid != 1 {
		t.Fatalf("insert Bob = RID %d, %v, want RID 1", rid, err)
	}
	if got, err := GetRecord(file, rid); err != nil || got != bob {
		t.Fatalf("GetRecord(%d) = %+v, %v, want %+v", rid, got, err, bob)
	}
	if err := DeleteRecord(file, rid); err != nil {
		t.Fatal(err)
	}
	for _, missing := range []int64{rid, -1, 2, math.MaxInt64} {
		if _, err := GetRecord(file, missing); !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("GetRecord(%d) error = %v, want ErrRecordNotFound", missing, err)
		}
	}

	dana := User{ID: 81, Name: "Dana", Age: 34}
	if reuse, err := InsertRecord(file, dana); err != nil || reuse != rid {
		t.Fatalf("insert Dana = RID %d, %v, want RID %d", reuse, err, rid)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, _, err := OpenDatabaseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if got, err := GetRecord(reopened, rid); err != nil || got != dana {
		t.Fatalf("GetRecord(%d) after reopen = %+v, %v, want %+v", rid, got, err, dana)
	}
}
