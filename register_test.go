// +build linux

package iouring

import (
	"io/ioutil"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterBuffers(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)
	vecs := make([]*syscall.Iovec, 10)
	require.NoError(t, RegisterBuffers(r.Fd(), vecs))
	require.NoError(t, UnregisterBuffers(r.Fd(), vecs))
	// require.Error(t, RegisterBuffers(-1, vecs))
	// require.Error(t, UnregisterBuffers(-1, vecs))
}

func TestFileRegistry(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	reg := NewFileRegistry(r.Fd())
	f, err := ioutil.TempFile("", "test-file-registry")
	require.NoError(t, err)
	f2, err := ioutil.TempFile("", "test-file-registry")
	require.NoError(t, err)

	require.NoError(t, reg.Register(int(f.Fd())))
	require.NoError(t, reg.Register(int(f2.Fd())))
	id, ok := reg.ID(int(f2.Fd()))
	require.NotZero(t, id)
	require.True(t, ok)
	require.NoError(t, reg.Unregister(int(f.Fd())))
}
