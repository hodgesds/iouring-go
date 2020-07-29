// +build linux

package iouring

import (
	"io"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPollAdd(t *testing.T) {
	r, err := New(2048, nil, WithID(1000000), WithEnterErrHandler(func(err error) { require.NoError(t, err) }))
	require.NoError(t, err)
	require.NotNil(t, r)

	data := []byte("foo")
	buf := make([]byte, len(data))
	pipeFds := make([]int, 2)
	require.NoError(t, unix.Pipe(pipeFds))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var count int
		for {
			require.NoError(t, r.PollAdd(pipeFds[0], POLLIN))
			if count == 3 {
				syscall.Close(pipeFds[1])
				break
			}
			syscall.Read(pipeFds[0], buf)
		}
		count++
	}()
	for {
		_, err = syscall.Write(pipeFds[1], data)
		if err == io.EOF {
			break
		}
	}
}
