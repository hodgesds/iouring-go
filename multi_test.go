// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiWriter(t *testing.T) {
	r, err := New(2048, &Params{
		Features: FeatNoDrop,
	})
	require.NoError(t, err)
	require.NotNil(t, r)

	f1, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f1.Name())

	f2, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f2.Name())

	w, err := r.MultiFileWriter(f1, f2)
	require.NoError(t, err)
	require.NotNil(t, w)

	content := []byte("testing...1,2.3")
	n, err := w.Write(content)
	require.NoError(t, err)
	require.Equal(t, n, len(content)*2)
}
