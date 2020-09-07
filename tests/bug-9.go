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

	// Open a file for registering with the ring.
	f, err := os.OpenFile("test", os.O_RDWR, 0755)
	if err != nil {
		log.Fatal(err)
	}

	// Register the file with the ring, which returns an io.WriteCloser.
	rw, err := r.FileReadWriter(f)
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, 4*1024*1024)
	//if _, err := rw.WriteAt(buf, 4328583168); err != nil {
	if _, err := rw.WriteAt(buf, 4096); err != nil {
		log.Fatal(err)
	}
}
