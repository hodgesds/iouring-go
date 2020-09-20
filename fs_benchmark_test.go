// +build linux

package iouring

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func createStatTestDir(depth, dentries int) (string, error) {
	dir, err := ioutil.TempDir("", "example")
	if err != nil {
		return "", err
	}
	for d := dentries; d > 0; d-- {
		_, err = ioutil.TempFile("", "example")
		if err != nil {
			return "", err
		}
	}
	if depth > 0 {
		_, err = createStatTestDir(depth-1, dentries)
		if err != nil {
			return "", err
		}
	}
	return dir, nil
}

func dropCache() error {
	return ioutil.WriteFile("/proc/sys/vm/drop_caches", []byte("2"), 0600)
}

func BenchmarkRingStatx(b *testing.B) {
	b.Skip("implement statx benchmark")
	tests := []struct {
		ringSize uint
		depth    int
		dentries int
	}{
		{
			ringSize: 1024,
			depth:    0,
			dentries: 10,
		},
	}

	for _, test := range tests {
		b.Run(
			fmt.Sprintf("ring-%d-statx-%d-%d", test.ringSize, test.depth, test.dentries),
			func(b *testing.B) {
				r, err := New(test.ringSize, &Params{
					Features: FeatNoDrop,
				})
				require.NoError(b, err)
				require.NotNil(b, r)
				defer r.Stop()

				dir, err := createStatTestDir(test.depth, test.dentries)
				defer os.RemoveAll(dir)
				require.NoError(b, err)

				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					require.NoError(b, dropCache())
					b.StartTimer()
				}
			},
		)
	}
}

func BenchmarkStatx(b *testing.B) {
	b.Skip("implement statx benchmark")
	tests := []struct {
		depth    int
		dentries int
	}{
		{
			depth:    0,
			dentries: 10,
		},
	}

	for _, test := range tests {
		b.Run(
			fmt.Sprintf("statx-%d-%d", test.depth, test.dentries),
			func(b *testing.B) {
				dir, err := createStatTestDir(test.depth, test.dentries)
				require.NoError(b, err)
				defer os.RemoveAll(dir)

				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					require.NoError(b, dropCache())
					b.StartTimer()
				}
			},
		)
	}
}
