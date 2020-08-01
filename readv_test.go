// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareReadv(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	data := []byte("testing...1,2,3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, err = f.Write(data)

	require.NoError(t, err)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	v := make([]*syscall.Iovec, 1)
	id, err := r.PrepareReadv(int(f.Fd()), v, 0)
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}
