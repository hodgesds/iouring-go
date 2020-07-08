// +build linux

package iouring

import (
	"io"
	"os"
)

// MultiFileWriter works just like io.MultiWriter but is io_uring backed and
// all writes will be submitted in a single ring enter.
func (r *Ring) MultiFileWriter(files ...*os.File) (io.Writer, error) {
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
	for i, f := range m.fios {
		id, err := f.prepareWrite(b)
		if err != nil {
			return 0, err
		}
		ids[i] = id
	}

	// The first entry submits all the requests.
	n, err := m.fios[0].getCqe(ids[0], len(ids), len(ids))
	if err != nil || len(ids) == 1 {
		return n, err
	}

	for i := 1; i < len(ids); i++ {
		// When multiple SQEs are submitted that point to the same go
		// address, in this case the byte slice "b" io_uring seems to
		// ignore the user_data field even if the FDs are different. To
		// handle this we use the user data field from the first SQE
		// for checking for completions.
		ni, err := m.fios[i].getCqe(ids[0], 0, 0)
		if err != nil {
			return n, err
		}
		n += ni
	}
	return n, nil
}
