package rpmdb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knqyf263/go-rpmdb/pkg/ndb"
)

func FuzzNDBListPackages(f *testing.F) {
	data, err := os.ReadFile("testdata/sle15-bci/Packages.db")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		tmpFile := filepath.Join(t.TempDir(), "Packages.db")
		if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
			return
		}

		db, err := ndb.Open(tmpFile)
		if err != nil {
			return
		}
		defer db.Close()

		rpmDB := &RpmDB{db: db}
		// Errors are expected on fuzzed input; we're looking for panics.
		rpmDB.ListPackages()
	})
}
