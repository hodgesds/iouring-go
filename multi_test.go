// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiWriter(t *testing.T) {
	t.Skip()
	r, err := New(2048, &Params{Features: FeatNoDrop})
	require.NoError(t, err)
	require.NotNil(t, r)

	f1, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f1.Name())

	f2, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f2.Name())

	f3, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f2.Name())

	w, err := r.MultiFileWriter(f1, f2, f3)
	require.NoError(t, err)
	require.NotNil(t, w)

	content := []byte("testing...1,2.3")
	n, err := w.Write(content)
	require.NoError(t, err)
	require.Equal(t, n, len(content)*3)

	// Test content of the three files.
	c := make([]byte, len(content))
	_, err = f1.Seek(0, 0)
	require.NoError(t, err)
	n, err = f1.Read(c)
	require.NoError(t, err)
	require.Equal(t, n, len(c))
	require.Equal(t, content, c)

	c = make([]byte, len(content))
	_, err = f2.Seek(0, 0)
	require.NoError(t, err)
	n, err = f2.Read(c)
	require.NoError(t, err)
	require.Equal(t, n, len(c))
	require.Equal(t, content, c)

	c = make([]byte, len(content))
	_, err = f3.Seek(0, 0)
	require.NoError(t, err)
	n, err = f3.Read(c)
	require.NoError(t, err)
	require.Equal(t, n, len(c))
	require.Equal(t, content, c)

	require.NoError(t, w.Close())
}
