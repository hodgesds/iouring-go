// +build linux

package iouring

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkWrite(b *testing.B) {
	tests := []struct {
		ringSize  uint
		writeSize int
	}{
		{
			ringSize:  1024,
			writeSize: 128,
		},
		{
			ringSize:  1024,
			writeSize: 512,
		},
		{
			ringSize:  1024,
			writeSize: 1024,
		},
		{
			ringSize:  1024,
			writeSize: 2048,
		},
		{
			ringSize:  1024,
			writeSize: 4096,
		},
	}

	for _, test := range tests {
		benchmarkRingWrite(b, test.ringSize, test.writeSize)
		benchmarkFileWrite(b, test.writeSize)
	}
}

func benchmarkRingWrite(b *testing.B, ringSize uint, writeSize int) {
	b.Run(
		fmt.Sprintf("ring-%d-write-%d", ringSize, writeSize),
		func(b *testing.B) {
			r, err := New(ringSize, nil)
			require.NoError(b, err)
			require.NotNil(b, r)
			data := make([]byte, writeSize)
			rand.Read(data)

			f, err := ioutil.TempFile("", "example")
			require.NoError(b, err)
			defer os.Remove(f.Name())

			rw, err := r.FileReadWriter(f)
			require.NoError(b, err)

			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rw.Write(data)
			}
		},
	)
}

func benchmarkFileWrite(b *testing.B, writeSize int) {
	b.Run(
		fmt.Sprintf("os-file-write-%d", writeSize),
		func(b *testing.B) {
			data := make([]byte, writeSize)
			n, err := rand.Read(data)
			require.NoError(b, err)
			require.Equal(b, writeSize, int(n))

			f, err := os.OpenFile(
				fmt.Sprintf("os-file-write-%d.test", writeSize),
				syscall.O_DIRECT|os.O_RDWR|os.O_CREATE, 0644)
			require.NoError(b, err)
			defer os.Remove(f.Name())

			b.SetBytes(int64(len(data)))
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				f.Write(data)
			}
		},
	)
}
