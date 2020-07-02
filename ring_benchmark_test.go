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
			ringSize:  8192,
			writeSize: 2048,
		},
		{
			ringSize:  8192,
			writeSize: 4096,
		},
	}

	for _, test := range tests {
		benchmarkFileWrite(b, test.writeSize)
		benchmarkRingWrite(b, test.ringSize, test.writeSize)
		//benchmarkRingDeadlineWrite(b, test.ringSize, test.writeSize)
	}
}

func benchmarkRingWrite(b *testing.B, ringSize uint, writeSize int) {
	b.Run(
		fmt.Sprintf("ring-%d-write-%d", ringSize, writeSize),
		func(b *testing.B) {
			r, err := New(ringSize, &Params{
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

			data := make([]byte, writeSize)

			b.SetBytes(int64(writeSize))
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

func benchmarkRingDeadlineWrite(b *testing.B, ringSize uint, writeSize int) {
	b.Run(
		fmt.Sprintf("ring-%d-deadlinewrite-%d", ringSize, writeSize),
		func(b *testing.B) {
			r, err := New(ringSize, &Params{Features: FeatNoDrop}, WithDeadline(100*time.Microsecond))
			require.NoError(b, err)
			require.NotNil(b, r)

			f, err := ioutil.TempFile("", "example2")
			require.NoError(b, err)
			defer os.Remove(f.Name())

			rw, err := r.FileReadWriter(f)
			require.NoError(b, err)

			data := make([]byte, writeSize)

			b.SetBytes(int64(writeSize))
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rw.Write(data)
				//_, err = rw.Write(data)
				//if err != nil {
				//	b.Fatal(err)
				//}
			}
		},
	)
}
