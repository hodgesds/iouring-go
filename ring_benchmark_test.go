// +build linux

package iouring

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func benchmarkRingWrite(b *testing.B, ringSize uint, writeSize int) {
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
			ringSize:  8192,
			writeSize: 2048,
		},
		{
			ringSize:  8192,
			writeSize: 4096,
		},
	}

	for _, test := range tests {
		b.Run(
			fmt.Sprintf("ring-%d-write-%d", test.ringSize, test.writeSize),
			func(b *testing.B) {
				r, err := New(test.ringSize, &Params{
					Features: FeatNoDrop,
				},
				)
				require.NoError(b, err)
				require.NotNil(b, r)

				//bufPool := sync.Pool{
				//	New: func() interface{} {
				//		return make([]byte, writeSize)
				//	},
				//}

				f, err := ioutil.TempFile("", "example")
				require.NoError(b, err)
				defer os.Remove(f.Name())

				rw, err := r.FileReadWriter(f)
				require.NoError(b, err)

				data := make([]byte, test.writeSize)

				b.SetBytes(int64(test.writeSize))
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					//data := bufPool.Get().([]byte)
					_, err = rw.Write(data)
					if err != nil {
						b.Fatal(err)
					}
					//bufPool.Put(data)
				}
			},
		)
	}
}

func BenchmarkFileWrite(b *testing.B) {
	tests := []struct {
		ringSize   uint
		writeSize  int
		multiwrite int
	}{
		{
			ringSize:   1024,
			writeSize:  128,
			multiwrite: 1,
		},
		{
			ringSize:   1024,
			writeSize:  512,
			multiwrite: 1,
		},
		{
			ringSize:   1024,
			writeSize:  1024,
			multiwrite: 1,
		},
		{
			ringSize:   8192,
			writeSize:  2048,
			multiwrite: 2,
		},
		{
			ringSize:   8192,
			writeSize:  4096,
			multiwrite: 2,
		},
	}
	for _, test := range tests {
		b.Run(
			fmt.Sprintf("os-file-write-%d", test.writeSize),
			func(b *testing.B) {
				data := make([]byte, test.writeSize)
				n, err := rand.Read(data)
				require.NoError(b, err)
				require.Equal(b, test.writeSize, int(n))

				f, err := os.OpenFile(
					fmt.Sprintf("os-file-write-%d.test", test.writeSize),
					os.O_RDWR|os.O_CREATE, 0644)
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
}

func BenchmarkRingDeadlineWrite(b *testing.B) {
	tests := []struct {
		ringSize  uint
		writeSize int
		deadline  time.Duration
	}{
		{
			ringSize:  1024,
			writeSize: 128,
			deadline:  1 * time.Millisecond,
		},
		{
			ringSize:  1024,
			writeSize: 512,
			deadline:  1 * time.Millisecond,
		},
		{
			ringSize:  1024,
			writeSize: 1024,
			deadline:  1 * time.Millisecond,
		},
		{
			ringSize:  8192,
			writeSize: 2048,
			deadline:  1 * time.Millisecond,
		},
		{
			ringSize:  8192,
			writeSize: 4096,
			deadline:  1 * time.Millisecond,
		},
	}
	for _, test := range tests {
		b.Run(
			fmt.Sprintf("ring-%d-deadline-%v-%d", test.ringSize, test.deadline.String(), test.writeSize),
			func(b *testing.B) {
				r, err := New(test.ringSize, &Params{Features: FeatNoDrop}, WithDeadline(test.deadline))
				require.NoError(b, err)
				require.NotNil(b, r)

				f, err := ioutil.TempFile("", "example")
				require.NoError(b, err)
				defer os.Remove(f.Name())

				rw, err := r.FileReadWriter(f)
				require.NoError(b, err)

				data := make([]byte, test.writeSize)

				b.SetBytes(int64(test.writeSize))
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err = rw.Write(data)
					if err != nil {
						b.Fatal(err)
					}
				}
			},
		)
	}
}

func BenchmarkRingMultiWrite(b *testing.B) {
	tests := []struct {
		ringSize   uint
		writeSize  int
		multiwrite int
	}{
		{
			ringSize:   1024,
			writeSize:  128,
			multiwrite: 1,
		},
		{
			ringSize:   1024,
			writeSize:  512,
			multiwrite: 1,
		},
		{
			ringSize:   1024,
			writeSize:  1024,
			multiwrite: 1,
		},
		{
			ringSize:   8192,
			writeSize:  2048,
			multiwrite: 2,
		},
		{
			ringSize:   8192,
			writeSize:  4096,
			multiwrite: 2,
		},
	}
	for _, test := range tests {
		b.Run(
			fmt.Sprintf("ring-%d-multiwrite%d-%d", test.ringSize, test.multiwrite, test.writeSize),
			func(b *testing.B) {
				r, err := New(test.ringSize, &Params{Features: FeatNoDrop})
				require.NoError(b, err)
				require.NotNil(b, r)

				files := make([]*os.File, test.multiwrite)
				for i := 0; i < test.multiwrite; i++ {
					f, err := ioutil.TempFile("", "example")
					require.NoError(b, err)
					defer os.Remove(f.Name())
					files[i] = f
				}
				w, err := r.MultiFileWriter(files...)
				require.NoError(b, err)

				data := make([]byte, test.writeSize)

				b.SetBytes(int64(test.writeSize * test.multiwrite))
				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err = w.Write(data)
					if err != nil {
						b.Fatal(err)
					}
				}
			},
		)
	}
}
