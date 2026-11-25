package minidb

import (
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
