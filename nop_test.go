// +build linux

package iouring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareNop(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	id, err := r.PrepareNop()
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}

func BenchmarkNop(b *testing.B) {
	r, err := New(2048, nil)
	require.NoError(b, err)
	require.NotNil(b, r)

	for i := 0; i < b.N; i++ {
		err = r.Nop()
		if err != nil {
			b.Fatal(err)
		}
	}
}
