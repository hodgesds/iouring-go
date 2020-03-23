// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRingFileReadWriter(t *testing.T) {
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

	rw := r.FileReadWriter(f)

	buf := make([]byte, len(content))
	_, err = rw.Read(buf)
	require.NoError(t, err)
	//require.True(t, n > 0)
	require.Contains(t, content, buf)
	require.NoError(t, rw.Close())
}
