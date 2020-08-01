// +build linux

package iouring

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareRecvmsg(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	id, err := r.PrepareRecvmsg(1, &syscall.Msghdr{}, 0)
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}
