// +build linux

package iouring

import (
	"testing"
	"time"

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

func BenchmarkPrepareNop(b *testing.B) {
	r, err := New(2048, nil)
	require.NoError(b, err)
	require.NotNil(b, r)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = r.PrepareNop()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNop(b *testing.B) {
	r, err := New(2048, nil)
	require.NoError(b, err)
	require.NotNil(b, r)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = r.Nop()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNopDeadline(b *testing.B) {
	r, err := New(2048, nil, WithDeadline(100*time.Microsecond))
	require.NoError(b, err)
	require.NotNil(b, r)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = r.Nop()
		if err != nil {
			b.Fatal(err)
		}
	}
}
