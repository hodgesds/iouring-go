// +build linux

package iouring

import (
	"crypto/rand"
	"io/ioutil"
	"os"
	"sync"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/require"
)

func TestSplice(t *testing.T) {
	out, err := ioutil.TempFile("", "out")
	require.NoError(t, err)
	defer os.Remove(out.Name())

	data := make([]byte, 32)
	n, err := rand.Read(data)
	require.NoError(t, err)
	require.Equal(t, n, 32)

	pipeFds := make([]int, 2)
	require.NoError(t, unix.Pipe(pipeFds))

	var wg sync.WaitGroup
	wg.Add(1)
	wrote := make(chan struct{})
	go func() {
		<-wrote
		defer wg.Done()
		c, err := unix.Splice(
			pipeFds[0], nil,
			int(out.Fd()), nil,
			32,
			unix.SPLICE_F_MOVE,
		)
		require.NoError(t, err)
		require.Equal(t, int(c), 32)
	}()

	syscall.Write(pipeFds[1], data)
	wrote <- struct{}{}
	syscall.Close(pipeFds[1])
	wg.Wait()
}

func TestRingSplice(t *testing.T) {
	t.Skip("FIX ME")
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	out, err := ioutil.TempFile("", "out")
	require.NoError(t, err)
	defer os.Remove(out.Name())

	data := make([]byte, 32)
	n, err := rand.Read(data)
	require.NoError(t, err)
	require.Equal(t, n, 32)

	pipeFds := make([]int, 2)
	require.NoError(t, unix.Pipe(pipeFds))

	var wg sync.WaitGroup
	wg.Add(1)
	wrote := make(chan struct{})
	go func() {
		<-wrote
		defer wg.Done()
		c, err := r.Splice(
			pipeFds[0], nil,
			int(out.Fd()), nil,
			32,
			unix.SPLICE_F_MOVE,
		)
		require.NoError(t, err)
		require.Equal(t, int(c), 32)
	}()

	syscall.Write(pipeFds[1], data)
	wrote <- struct{}{}
	syscall.Close(pipeFds[1])
	wg.Wait()
}
