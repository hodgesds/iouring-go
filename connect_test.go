// +build linux

package iouring

import (
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareConnect(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	fd, err := syscall.Socket(
		syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	require.NoError(t, err)
	require.True(t, fd > 0)
	addr := &syscall.SockaddrInet4{
		Port: 80,
	}
	copy(addr.Addr[:], net.ParseIP("8.8.8.8"))
	id, err := r.PrepareConnect(
		fd,
		addr,
		syscall.SizeofSockaddrInet4,
	)
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}
