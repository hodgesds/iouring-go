// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRingStatx(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	path, err := os.Getwd()
	require.NoError(t, err)

	f, err := ioutil.TempFile(path, "statx")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.Write([]byte("test"))
	require.NoError(t, err)

	var (
		x1 unix.Statx_t
		x2 unix.Statx_t
	)
	d, err := os.Open(path)
	require.NoError(t, err)
	defer d.Close()

	err = r.Statx(int(d.Fd()), path, 0, unix.STATX_ALL, &x1)
	require.NoError(t, err)

	err = unix.Statx(int(d.Fd()), path, 0, unix.STATX_ALL, &x2)
	require.NoError(t, err)
	require.Equal(t, x1, x2)
}

func BenchmarkStatxRing(b *testing.B) {
	r, err := New(2048, nil)
	require.NoError(b, err)
	require.NotNil(b, r)

	path, err := os.Getwd()
	require.NoError(b, err)

	f, err := ioutil.TempFile(path, "statx")
	require.NoError(b, err)
	defer os.Remove(f.Name())

	_, err = f.Write([]byte("test"))
	require.NoError(b, err)

	var x1 unix.Statx_t
	d, err := os.Open(path)
	require.NoError(b, err)
	defer d.Close()

	for i := 0; i < b.N; i++ {
		err = r.Statx(int(d.Fd()), path, 0, unix.STATX_ALL, &x1)
		if err != nil {
			b.Fatal(err)
		}
	}
}
