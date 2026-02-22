package rpmdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//
func Test_headerImport(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			// found by fuzzer
			name: "negative il",
			data: []byte{0xe3, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, _ = headerImport(tt.data)
			})
		})
	}
}

func Test_headerImport_ilOverflow(t *testing.T) {
	tests := []struct {
		name string
		// il value encoded as big-endian int32, followed by dl=1
		il [4]byte
	}{
		{
			// il*16 overflows int32, bypassing the pvlen size check
			name: "int32 overflow",
			il:   [4]byte{0x08, 0x00, 0x00, 0x01},
		},
		{
			// il just below int32 max; would allocate 32GB without a guard
			name: "max int32",
			il:   [4]byte{0x7F, 0xFF, 0xFF, 0xFF},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := append(tt.il[:], 0x00, 0x00, 0x00, 0x01)
			_, err := headerImport(data)
			assert.ErrorContains(t, err, "too large")
		})
	}
}
