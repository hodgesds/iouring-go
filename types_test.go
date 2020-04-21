// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRingFileReadWriterRead(t *testing.T) {
	r, err := New(1024)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2.3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.Write(content)
	require.NoError(t, err)
	require.NoError(t, f.Sync())

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	buf := make([]byte, len(content))
	n, err := rw.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Equal(t, content, buf)
	require.NoError(t, rw.Close())
}

func TestRingFileReadWriterWrite(t *testing.T) {
	r, err := New(1024)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2.3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	// Write to the file using the ring
	_, err = rw.Write(content)
	require.NoError(t, err)

	require.NoError(t, f.Sync())

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	buf := make([]byte, len(content))
	n, err := f.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Equal(t, content, buf)
	require.NoError(t, rw.Close())
}

func TestRingFileReadWriterWriteRead(t *testing.T) {
	r, err := New(1024)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2.3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	// Write to the file using the ring
	_, err = rw.Write(content)
	require.NoError(t, err)

	require.NoError(t, f.Sync())

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	// Seek from the file breaks the ReadWriter because the offset is
	// stored internally on the ReadWriter, so create a new one.
	rw, err = r.FileReadWriter(f)
	require.NoError(t, err)

	buf := make([]byte, len(content))
	n, err := rw.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Equal(t, content, buf)
	require.NoError(t, rw.Close())
}
