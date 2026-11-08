package cbpf

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDenyList_Encodes(t *testing.T) {
	tests := []struct {
		name  string
		arch  uint32
		nrs   []int
		wants []inst
	}{
		{
			"one",
			0xc00000b7,
			[]int{165},
			[]inst{
				{ldAbs, 0, 0, 4},
				{jeq, 0, 3, 0xc00000b7},
				{ldAbs, 0, 0, 0},
				{jeq, 0, 1, 165},
				{ret, 0, 0, retErrno | errnoPerm},
				{ret, 0, 0, retAllow},
			},
		},
		{
			"sorted",
			0x80000016,
			[]int{321, 165},
			[]inst{
				{ldAbs, 0, 0, 4},
				{jeq, 0, 5, 0x80000016},
				{ldAbs, 0, 0, 0},
				{jeq, 0, 1, 165},
				{ret, 0, 0, retErrno | errnoPerm},
				{jeq, 0, 1, 321},
				{ret, 0, 0, retErrno | errnoPerm},
				{ret, 0, 0, retAllow},
			},
		},
		{
			"none allows all",
			0xc000003e, nil,
			[]inst{
				{ldAbs, 0, 0, 4},
				{jeq, 0, 1, 0xc000003e},
				{ldAbs, 0, 0, 0},
				{ret, 0, 0, retAllow},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog := renderDenyList(tt.arch, tt.nrs)

			require.Len(t, prog, 8*len(tt.wants))
			for i, want := range tt.wants {
				assert.Equal(t, want, decodeInst(t, prog, i), "instruction %d", i)
			}
		})
	}
}

// inst is a decoded sock_filter: code, its two jump offsets, and the constant.
type inst struct {
	code uint16
	jt   uint8
	jf   uint8
	k    uint32
}

func decodeInst(t *testing.T, prog []byte, i int) inst {
	t.Helper()
	off := i * 8
	return inst{
		code: binary.NativeEndian.Uint16(prog[off:]),
		jt:   prog[off+2],
		jf:   prog[off+3],
		k:    binary.NativeEndian.Uint32(prog[off+4:]),
	}
}
