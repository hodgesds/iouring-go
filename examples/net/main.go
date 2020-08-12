package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/hodgesds/iouring-go"
)

var (
	port  int
	debug bool
)

func init() {
	flag.IntVar(&port, "port", 9999, "HTTP port")
	flag.BoolVar(&debug, "debug", false, "debug mode")
}

func main() {
	flag.Parse()
	ops := []iouring.RingOption{
		iouring.WithEnterErrHandler(func(err error) {
			log.Println(err)
		}),
	}
	if debug {
		ops = append(ops, iouring.WithDebug())
	}
	r, err := iouring.New(
		8192,
		&iouring.Params{
			Features: iouring.FeatNoDrop,
		},
		ops...,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("listening on port: %d\n", port)
	l, err := r.SockoptListener(
		"tcp",
		fmt.Sprintf(":%d", port),
		func(err error) {
			log.Println(err)
		},
		iouring.SOReuseport,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// The "/" pattern matches everything, so we need to check
		// that we're at the root here.
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		fmt.Fprintf(w, "hello io_uring!\n")
	})

	s := http.Server{Handler: mux}
	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
