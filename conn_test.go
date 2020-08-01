package iouring

import (
	"bytes"
	"io/ioutil"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSockoptListener(t *testing.T) {
	t.Skip("skip")
	r, err := New(8192, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	l, err := r.SockoptListener("tcp", ":9822", nil)
	require.NoError(t, err)
	require.NotNil(t, l)

	go func() {
		conn2, err := net.Dial("tcp", ":9822")
		require.NoError(t, err)
		require.NotNil(t, conn2)
		require.NoError(t, conn2.Close())
	}()
	conn, err := l.Accept()
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Close())
}

func TestFastOpenAllowed(t *testing.T) {
	b, err := ioutil.ReadFile("/proc/sys/net/ipv4/tcp_fack")
	require.NoError(t, err)
	if bytes.Contains(b, []byte("3")) {
		require.NoError(t, FastOpenAllowed())
	} else {
		require.Error(t, FastOpenAllowed())
	}
}
