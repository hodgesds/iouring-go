// +build linux

package iouring

import (
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRingFileReadWriterRead(t *testing.T) {
	r, err := New(1024, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2.3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	_, err = f.Write(content)
	require.NoError(t, err)
	require.NoError(t, f.Sync())

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	buf := make([]byte, len(content)/2)
	n, err := rw.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Subset(t, content, buf)

	buf = make([]byte, len(content)/2)
	n, err = rw.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Subset(t, content, buf)

	require.NoError(t, rw.Close())
}

func TestRingFileReadWriterSeek(t *testing.T) {
	r, err := New(1024, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2,3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	_, err = rw.Write(content)
	require.NoError(t, err)
	require.NoError(t, f.Sync())

	_, err = rw.Seek(0, io.SeekCurrent)
	require.NoError(t, err)
	_, err = rw.Seek(0, io.SeekEnd)
	require.NoError(t, err)
}

func TestRingFileReadWriterReadAt(t *testing.T) {
	r, err := New(1024, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2,3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	_, err = rw.Write(content)
	require.NoError(t, err)
	require.NoError(t, f.Sync())

	buf := make([]byte, len(content))
	_, err = rw.ReadAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, content, buf)
}

func TestRingFileReadWriterWriteAt(t *testing.T) {
	r, err := New(1024, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2,3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	_, err = rw.WriteAt(content, 0)
	require.NoError(t, err)
	require.NoError(t, f.Sync())

	buf := []byte("testing...3,2,1")
	n, err := rw.WriteAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, len(buf), n)

	buf2 := make([]byte, len(buf))
	_, err = rw.ReadAt(buf2, 0)
	require.NoError(t, err)
	require.Equal(t, buf, buf2)
}

func TestRingFileReadWriterWrite(t *testing.T) {
	r, err := New(1024, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2.3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	// Write to the file using the ring
	_, err = rw.Write(content)
	require.NoError(t, err)

	require.NoError(t, f.Sync())

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	buf := make([]byte, len(content))
	n, err := f.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Equal(t, content, buf)
	require.NoError(t, rw.Close())
}

func TestRingFileReadWriterWriteRead(t *testing.T) {
	r, err := New(1024, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2,3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	// Write to the file using the ring
	_, err = rw.Write(content)
	require.NoError(t, err)

	require.NoError(t, f.Sync())

	_, err = rw.Seek(0, 0)
	require.NoError(t, err)

	buf := make([]byte, len(content))
	n, err := rw.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Equal(t, content, buf)

	require.NoError(t, rw.Close())
}

func TestRingReadWrap(t *testing.T) {
	ringSize := uint(8)
	r, err := New(ringSize, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := os.Open("/dev/zero")
	require.NoError(t, err)

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	for i := 0; i < int(ringSize)*100; i++ {
		buf := make([]byte, 8)
		n, err := rw.Read(buf)
		require.NoError(t, err)
		require.True(t, n > 0)
	}
}

func TestConcurrentReaders(t *testing.T) {
	ringSize := uint(8)
	r, err := New(ringSize, &Params{})
	require.NoError(t, err)
	require.NotNil(t, r)

	f, err := os.Open("/dev/zero")
	require.NoError(t, err)

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	work := make(chan struct{})
	stop := make(chan struct{})
	done := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case <-stop:
					wg.Done()
					return
				case <-work:
					buf := make([]byte, 1)
					_, err := rw.Read(buf)
					if err != nil && err != ErrEntryNotFound {
						require.NoError(t, err)
					}
					done <- struct{}{}
				}
			}
		}()
	}

	for i := 0; i < int(ringSize+2); i++ {
		work <- struct{}{}
		<-done
	}
	close(stop)
	wg.Wait()
}

func TestCqeIsZero(t *testing.T) {
	cqe := &CompletionEntry{}
	require.True(t, cqe.IsZero())
	cqe.Res = 1
	require.False(t, cqe.IsZero())
}

func TestReadWriterEOF(t *testing.T) {
	r, err := New(1024, nil)
	require.NoError(t, err)
	require.NotNil(t, r)

	content := []byte("testing...1,2,3")
	f, err := ioutil.TempFile("", "example")
	require.NoError(t, err)

	rw, err := r.FileReadWriter(f)
	require.NoError(t, err)

	// Write to the file using the ring
	_, err = rw.Write(content)
	require.NoError(t, err)

	buf := make([]byte, 4096)
	_, err = rw.Read(buf)
	require.Error(t, err)
	f.Close()
	os.Remove(f.Name())
}
