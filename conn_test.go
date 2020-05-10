package iouring

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSockoptListener(t *testing.T) {
	r, err := New(8192, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	port := 9832
	l, err := r.SockoptListener("tcp", fmt.Sprintf(":%d", port))
	require.NoError(t, err)
	require.NotNil(t, l)

	go func() {
		conn2, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
		require.NoError(t, err)
		require.NotNil(t, conn2)
		require.NoError(t, conn2.Close())
	}()
	conn, err := l.Accept()
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
}
