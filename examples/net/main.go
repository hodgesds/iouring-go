package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/hodgesds/iouring-go"
)

var port int

func init() {
	flag.IntVar(&port, "port", 9999, "HTTP port")
}

func main() {
	flag.Parse()
	r, err := iouring.New(
		8192,
		&iouring.Params{},
		iouring.WithID(100000),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("listening on port: %d\n", port)
	l, err := r.SockoptListener(
		"tcp",
		fmt.Sprintf(":%d", port),
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

	s := http.Server{
		Handler: mux,
		//ReadTimeout:  1 * time.Second,
		//WriteTimeout: 1 * time.Second,
		ConnState: func(c net.Conn, s http.ConnState) {
			fmt.Printf("conn: %+v, %+v\n", c, s)
		},
	}
	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
