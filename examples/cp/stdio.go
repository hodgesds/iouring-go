package main

import (
	"flag"
	"io"
	"log"
	"os"

	"golang.org/x/sys/unix"
)

var bufSize int

func init() {
	flag.IntVar(&bufSize, "buf", 4096, "read buffer size")
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		log.Fatalf("expected src dst: %v", args)
	}

	src, err := os.Open(args[0])
	if err != nil {
		log.Fatalf("expected src dst: %v", args)
	}

	dst, err := os.OpenFile(args[1], os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}

	// First get size of src.
	//stat, err := src.Stat()
	//if err != nil {
	//	log.Fatal(err)
	//}

	// fadvise sequential read to EOF.
	if err := unix.Fadvise(int(src.Fd()), int64(0), int64(0), 3); err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, bufSize)
	for {
		n, err := src.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		_, err = dst.Write(buf[:n])
		if err != nil {
			log.Fatal(err)
		}
	}

	if err := src.Close(); err != nil {
		log.Fatal(err)
	}

	if err := dst.Close(); err != nil {
		log.Fatal(err)
	}
}
