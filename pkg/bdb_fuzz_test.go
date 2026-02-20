package rpmdb

import (
	"bytes"
	"os"
	"testing"

	"github.com/knqyf263/go-rpmdb/pkg/bdb"
)

func fuzzBDBListPackages(t *testing.T, data []byte) {
	if len(data) == 0 {
		return
	}

	db, err := bdb.NewReader(bytes.NewReader(data))
	if err != nil {
		return
	}
	defer db.Close()

	rpmDB := &RpmDB{db: db}
	// Errors are expected on fuzzed input; we're looking for panics.
	rpmDB.ListPackages()
}

func FuzzBDBListPackages(f *testing.F) {
	// Seed with the smallest BDB fixture to keep mutations manageable.
	// Larger fixtures (5-33MB) cause fuzzer subprocesses to OOM.
	data, err := os.ReadFile("testdata/libuuid/Packages")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzBDBListPackages(t, data)
	})
}
