// build linux

package iouring

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadWriterReadAt(t *testing.T) {
	r, err := New(1024, &Params{
		Features: FeatNoDrop,
	})
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2.3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.Write(content)
	require.NoError(t, err)
	require.NoError(t, f.Sync())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	buf := make([]byte, len(content)/2)
	n, err := rw.ReadAt(buf, 0)
	require.True(t,
		n == len(buf),
		"Excpected length %d, got: %d",
		n,
		len(buf),
	)
}

func TestReadWriterWriteAt(t *testing.T) {
	r, err := New(1024, &Params{
		Features: FeatNoDrop,
	})
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2.3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	n, err := rw.WriteAt(content, 0)
	require.True(t,
		n == len(content),
		"Excpected length %d, got: %d",
		n,
		len(content),
	)
}
