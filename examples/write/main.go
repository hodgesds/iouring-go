package main

import (
	"log"
	"os"

	"github.com/hodgesds/iouring-go"
)

func main() {
	r, err := iouring.New(1024, &iouring.Params{
		Features: iouring.FeatNoDrop,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Open a file for registring with the ring.
	f, err := os.OpenFile("hello.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal(err)
	}

	// Register the file with the ring, which returns an io.WriteCloser.
	rw, err := r.FileReadWriter(f)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := rw.Write([]byte("hello io_uring!")); err != nil {
		log.Fatal(err)
	}

	// Close the WriteCloser, which closes the open file (f).
	if err := r.Close(); err != nil {
		log.Fatal(err)
	}
}
