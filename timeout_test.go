// +build linux

package iouring

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareTimeout(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	id, err := r.PrepareTimeout(&syscall.Timespec{Sec: 1}, 1, 0)
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}

func TestPrepareTimeoutRemove(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	id, err := r.PrepareTimeoutRemove(0, 0)
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}
