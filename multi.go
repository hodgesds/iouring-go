// +build linux

package iouring

import (
	"fmt"
	"io"
	"os"
	"runtime"
)

// MultiFileWriter works just like io.MultiWriter but is io_uring backed and
// all writes will be submitted in a single ring enter.
func (r *Ring) MultiFileWriter(files ...*os.File) (io.WriteCloser, error) {
	fios := make([]*ringFIO, len(files))
	for i, f := range files {
		fio, err := r.fileReadWriter(f)
		if err != nil {
			return nil, err
		}
		fios[i] = fio
	}
	return &multiFileIO{
		fios: fios,
	}, nil
}

type multiFileIO struct {
	fios []*ringFIO
}

// Write implements the io.Writer interface.
func (m *multiFileIO) Write(b []byte) (int, error) {
	ids := make([]uint64, len(m.fios))
	var (
		err   error
		id    uint64
		ready func()
	)
	for i, f := range m.fios {
		//if i >= 1 {
		//	// Is copy needed for unique pointers?
		//	bb := make([]byte, len(b))
		//	copy(bb, b)
		//	id, err = f.prepareWrite(bb)
		//} else {
		id, ready, err = f.PrepareWrite(b, 0)
		//}
		if err != nil {
			return 0, err
		}
		ids[i] = id
		ready()
	}

	fmt.Printf("%+v\n", m.fios[0].r.sq.Entries[:5])
	fmt.Printf("%+v\n", m.fios[0].r.cq.Entries[:5])
	_, err = m.fios[0].r.Enter(uint(len(ids)), uint(len(ids)), EnterGetEvents, nil)
	//_, err = m.fios[0].r.Enter(uint(len(ids)), uint(len(ids)), 0, nil)
	if err != nil {
		return 0, err
	}
	fmt.Printf("%+v\n", m.fios[0].r.sq.Entries[:5])
	fmt.Printf("%+v\n", m.fios[0].r.cq.Entries[:5])
	fmt.Printf("%+v\n", m.fios[0])

	// The first entry submits all the requests.
	n, err := m.fios[0].getCqe(ids[0], 0, 0)
	if err != nil || len(ids) == 1 {
		return n, err
	}

	for i := 1; i < len(ids); i++ {
		fmt.Printf("submit: head: %v tail: %v\n", m.fios[0].r.SubmitHead(), m.fios[0].r.SubmitTail())
		fmt.Printf("complete: head: %v tail: %v\n", m.fios[0].r.CompleteHead(), m.fios[0].r.CompleteTail())
		fmt.Printf("%+v\n", m.fios[0].r.sq.Entries[:5])
		fmt.Printf("%+v\n", m.fios[i].r.cq.Entries[:5])
		// BUG: This is obviously a bug.
		// When multiple SQEs are submitted that point to the same go
		// address, in this case the byte slice "b" io_uring seems to
		// ignore the user_data field even if the FDs are different. To
		// handle this we use the user data field from the first SQE
		// for checking for completions.
		ni, err := m.fios[i].getCqe(ids[i], 0, 0)
		if err != nil {
			return n, err
		}
		n += ni
	}
	runtime.KeepAlive(b)
	return n, nil
}

// Close implements the io.ReadWriter interface
func (m *multiFileIO) Close() error {
	for _, f := range m.fios {
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}
