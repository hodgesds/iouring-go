package main

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/hodgesds/iouring-go"
)

var bufSize int

func init() {
	flag.IntVar(&bufSize, "buf", 4096, "read buffer size")
}

func main() {
	flag.Parse()
	ring, err := iouring.New(4096, &iouring.Params{
		Features: iouring.FeatNoDrop,
	})
	if err != nil {
		log.Fatal(err)
	}

	args := flag.Args()
	if len(args) != 2 {
		log.Fatal("expected src dst")
	}

	src, err := os.Open(args[0])
	if err != nil {
		log.Fatal(err)
	}

	dst, err := os.OpenFile(args[1], os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}

	r, err := ring.FileReadWriter(src)
	if err != nil {
		log.Fatal(err)
	}

	w, err := ring.FileReadWriter(dst)
	if err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, bufSize)

	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		if n == 0 {
			break
		}
		_, err = w.Write(buf[:n])
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := r.Close(); err != nil {
		log.Fatal(err)
	}

	if err := w.Close(); err != nil {
		log.Fatal(err)
	}
}
