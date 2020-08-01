// +build linux

package iouring

import (
	"io/ioutil"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareWritev(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	b := byte(1)
	v := &syscall.Iovec{
		Base: &b,
	}
	v.SetLen(1)
	iovs := []*syscall.Iovec{v}
	id, err := r.PrepareReadv(int(f.Fd()), iovs, 0)
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}
