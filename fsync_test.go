// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFsync(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := ioutil.TempFile("", "fsync")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	err = r.Fsync(int(f.Fd()), 0)
	require.NoError(t, err)
}
