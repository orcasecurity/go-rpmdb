package bdb

import (
	"io"
	"os"

	dbi "github.com/knqyf263/go-rpmdb/pkg/db"
	"golang.org/x/xerrors"
)

var validPageSizes = map[uint32]struct{}{
	512:   {},
	1024:  {},
	2048:  {},
	4096:  {},
	8192:  {},
	16384: {},
	32768: {},
	65536: {},
}

type BerkeleyDB struct {
	reader       io.ReadSeeker
	HashMetadata *HashMetadataPage
}

func Open(path string) (*BerkeleyDB, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	db, err := NewReader(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	return db, nil
}

func NewReader(r io.ReadSeeker) (*BerkeleyDB, error) {
	// read just a bit in to parse at least the metadata...
	metadataBuff := make([]byte, 512)
	_, err := r.Read(metadataBuff)
	if err != nil {
		return nil, xerrors.Errorf("failed to read metadata: %w", err)
	}

	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, xerrors.Errorf("failed to seek db file: %w", err)
	}

	hashMetadata, err := ParseHashMetadataPage(metadataBuff)
	if err != nil {
		return nil, err
	}

	if _, ok := validPageSizes[hashMetadata.PageSize]; !ok {
		return nil, xerrors.Errorf("unexpected page size: %+v", hashMetadata.PageSize)
	}

	// Validate LastPageNo against the actual reader size to prevent
	// excessive iteration on corrupted metadata.
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, xerrors.Errorf("failed to determine reader size: %w", err)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, xerrors.Errorf("failed to seek db file: %w", err)
	}
	maxPages := uint32(size / int64(hashMetadata.PageSize))
	if hashMetadata.LastPageNo > maxPages {
		return nil, xerrors.Errorf("LastPageNo %d exceeds data size (%d pages)", hashMetadata.LastPageNo, maxPages)
	}

	return &BerkeleyDB{
		reader:       r,
		HashMetadata: hashMetadata,
	}, nil
}

func (db *BerkeleyDB) Close() error {
	if closer, ok := db.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (db *BerkeleyDB) Read() <-chan dbi.Entry {
	entries := make(chan dbi.Entry)

	go func() {
		defer close(entries)

		// Track overflow pages visited across all chains to prevent
		// O(N^2) re-traversal of the same pages from different hash entries.
		overflowVisited := make(map[uint32]struct{})

		for pageNum := uint32(0); pageNum <= db.HashMetadata.LastPageNo; pageNum++ {
			pageData, err := slice(db.reader, int(db.HashMetadata.PageSize))
			if err != nil {
				// truncated last page is expected at the end of some databases
				break
			}

			// keep track of the start of the next page for the next iteration...
			endOfPageOffset, err := db.reader.Seek(0, io.SeekCurrent)
			if err != nil {
				entries <- dbi.Entry{
					Err: err,
				}
				return
			}

			hashPageHeader, err := ParseHashPage(pageData, db.HashMetadata.Swapped)
			if err != nil {
				entries <- dbi.Entry{
					Err: err,
				}
				return
			}

			if hashPageHeader.PageType != HashUnsortedPageType && // for RHEL/CentOS 5
				hashPageHeader.PageType != HashPageType {
				// skip over pages that do not have hash values
				continue
			}

			hashPageIndexes, err := HashPageValueIndexes(pageData, hashPageHeader.NumEntries, db.HashMetadata.Swapped)
			if err != nil {
				// skip pages with invalid index entries
				continue
			}

			for _, hashPageIndex := range hashPageIndexes {
				if int(hashPageIndex) >= len(pageData) {
					// skip entries with out-of-range indexes
					continue
				}

				// the first byte is the page type, so we can peek at it first before parsing further...
				valuePageType := pageData[hashPageIndex]

				// Only Overflow pages contain package data, skip anything else.
				if valuePageType != HashOffIndexPageType {
					continue
				}

				if int(hashPageIndex)+HashOffPageSize > len(pageData) {
					// skip entries that extend past the page boundary
					continue
				}

				// Traverse the page to concatenate the data that may span multiple pages.
				valueContent, err := HashPageValueContent(
					db.reader,
					pageData,
					hashPageIndex,
					db.HashMetadata.PageSize,
					db.HashMetadata.Swapped,
					overflowVisited,
				)

				entries <- dbi.Entry{
					Value: valueContent,
					Err:   err,
				}

				if err != nil {
					return
				}
			}

			// go back to the start of the next page for reading...
			_, err = db.reader.Seek(endOfPageOffset, io.SeekStart)
			if err != nil {
				entries <- dbi.Entry{
					Err: err,
				}
				return
			}
		}
	}()

	return entries
}
