package rpmdb

import (
	"os"
	"testing"
)

func FuzzHeaderImport(f *testing.F) {
	data, err := os.ReadFile("testdata/blob.bin")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		indexEntries, err := headerImport(data)
		if err != nil {
			return
		}

		// Also exercise getNEVRA and downstream parsing to catch
		// panics in package metadata extraction.
		pkg, err := getNEVRA(indexEntries)
		if err != nil {
			return
		}

		// Exercise file list building which does index arithmetic.
		pkg.InstalledFiles()
	})
}
