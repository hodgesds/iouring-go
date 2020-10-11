// +build linux

package iouring

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPrepareAccept(t *testing.T) {
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
	id, err := r.PrepareAccept(
		fd,
		addr,
		syscall.SizeofSockaddrInet4,
		0,
	)
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}

func TestClose(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := ioutil.TempFile("", "fsync")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	err = r.Close(int(f.Fd()))
	require.NoError(t, err)
}

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

func TestFadvise(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := ioutil.TempFile("", "fadvise")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	data := []byte("hello fadvise")
	_, err = f.Write(data)
	require.NoError(t, err)

	err = r.Fadvise(int(f.Fd()), 0, uint32(len(data)), unix.FADV_NORMAL)
	require.NoError(t, err)
}

func TestFallocate(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := ioutil.TempFile("", "fallocate")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	data := []byte("hello fallocate")
	_, err = f.Write(data)
	require.NoError(t, err)

	err = r.Fallocate(int(f.Fd()), unix.FALLOC_FL_KEEP_SIZE, 0, int64(len(data)))
	require.NoError(t, err)
}

func TestFsync(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := ioutil.TempFile("", "fsync")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	err = r.Fsync(int(f.Fd()), 0)
	require.NoError(t, err)
}

func TestPrepareNop(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	id, err := r.PrepareNop()
	require.NoError(t, err)
	require.True(t, id > uint64(0))
}

func BenchmarkPrepareNop(b *testing.B) {
	r, err := New(2048, nil)
	require.NoError(b, err)
	require.NotNil(b, r)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = r.PrepareNop()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNop(b *testing.B) {
	r, err := New(2048, nil)
	require.NoError(b, err)
	require.NotNil(b, r)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = r.Nop()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNopDeadline(b *testing.B) {
	tests := []struct {
		ringSize  uint
		writeSize int
		deadline  time.Duration
	}{
		{
			ringSize: 1024,
			deadline: 1 * time.Millisecond,
		},
		{
			ringSize: 1024,
			deadline: 100 * time.Microsecond,
		},
		{
			ringSize: 1024,
			deadline: 10 * time.Microsecond,
		},
	}
	for _, test := range tests {
		b.Run(
			fmt.Sprintf(
				"ring-%d-nop-deadline-%v",
				test.ringSize,
				test.deadline.String(),
			),
			func(b *testing.B) {
				r, err := New(
					test.ringSize,
					nil,
					WithDeadline(test.deadline),
				)
				require.NoError(b, err)
				require.NotNil(b, r)

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					err = r.Nop()
					if err != nil {
						b.Fatal(err)
					}
				}
			},
		)
	}
}

func TestPollAdd(t *testing.T) {
	t.Skip("FIX ME")
	r, err := New(
		2048,
		nil,
		WithID(1000000),
		WithEnterErrHandler(func(err error) { require.NoError(t, err) }),
	)
	require.NoError(t, err)
	require.NotNil(t, r)

	data := []byte("foo")
	buf := make([]byte, len(data))
	pipeFds := make([]int, 2)
	require.NoError(t, unix.Pipe2(pipeFds, syscall.O_NONBLOCK))
	var wg sync.WaitGroup
	wg.Add(1)
	ready := make(chan struct{})
	go func() {
		defer wg.Done()
		ready <- struct{}{}
		for i := 0; i < 3; i++ {
			println("F")
			syscall.Read(pipeFds[1], buf)
			ready <- struct{}{}
			println("y")
			require.NoError(t, r.PollAdd(pipeFds[1], POLLIN))
			println("D")
		}
		syscall.Close(pipeFds[0])
	}()
	for i := 0; i < 3; i++ {
		<-ready
		_, err = syscall.Write(pipeFds[0], data)
		println("W")
		if err == io.EOF {
			break
		}
	}
	wg.Wait()
}

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

func TestRingStatx(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	path, err := os.Getwd()
	require.NoError(t, err)

	f, err := ioutil.TempFile(path, "statx")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.Write([]byte("test"))
	require.NoError(t, err)

	var (
		x1 unix.Statx_t
		x2 unix.Statx_t
	)
	d, err := os.Open(path)
	require.NoError(t, err)
	defer d.Close()

	err = r.Statx(int(d.Fd()), path, 0, unix.STATX_ALL, &x1)
	require.NoError(t, err)

	err = unix.Statx(int(d.Fd()), path, 0, unix.STATX_ALL, &x2)
	require.NoError(t, err)
	require.Equal(t, x1, x2)
}

func TestSend(t *testing.T) {
	r, err := New(2048, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	sockFile := fmt.Sprintf("sock_test_%d.sock", rand.Int())

	l, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: sockFile,
		Net:  "unix",
	})
	require.NoError(t, err)
	defer l.Close()

	b := []byte("some bytes")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}

		exB := make([]byte, len(b))
		_, err = conn.Read(exB)
		require.NoError(t, err)
		require.Equal(t, b, exB)

		require.NoError(t, conn.Close())
	}()

	c, err := net.DialUnix("unix", nil, &net.UnixAddr{
		Name: sockFile,
		Net:  "unix",
	})
	require.NoError(t, err)
	f, err := c.File()
	require.NoError(t, err)
	require.NoError(t, r.Send(int(f.Fd()), b, 0))
	wg.Wait()
}

func BenchmarkStatxRing(b *testing.B) {
	r, err := New(2048, nil)
	require.NoError(b, err)
	require.NotNil(b, r)

	path, err := os.Getwd()
	require.NoError(b, err)

	f, err := ioutil.TempFile(path, "statx")
	require.NoError(b, err)
	defer os.Remove(f.Name())

	_, err = f.Write([]byte("test"))
	require.NoError(b, err)

	var x1 unix.Statx_t
	d, err := os.Open(path)
	require.NoError(b, err)
	defer d.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = r.Statx(int(d.Fd()), path, 0, unix.STATX_ALL, &x1)
		if err != nil {
			b.Fatal(err)
		}
	}
}

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
