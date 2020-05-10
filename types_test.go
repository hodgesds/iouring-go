// +build linux

package iouring

import (
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

	buf := make([]byte, len(content)/2)
	n, err := rw.Read(buf)
	require.NoError(t, err)
	require.True(t, n > 0)

	buf2 := make([]byte, len(content)/2+1)
	n, err = rw.Read(buf2)
	require.NoError(t, err)
	require.True(t, n > 0)
	require.Equal(t, content, append(buf, buf2...))

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

	for i := 0; i < int(ringSize)*2; i++ {
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
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		go func() {
			for {
				select {
				case <-stop:
					return
				case <-work:
					buf := make([]byte, 16)
					_, err := rw.Read(buf)
					wg.Done()
					if err != nil && err != ErrEntryNotFound {
						require.NoError(t, err)
					}
				}
			}
		}()
	}

	for i := 0; i < int(ringSize*2); i++ {
		wg.Add(1)
		work <- struct{}{}
	}

	wg.Wait()
	close(stop)
}
