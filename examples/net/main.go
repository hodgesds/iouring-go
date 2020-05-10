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
	r, err := iouring.New(8192, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("port: %d\n", port)
	l, err := r.SockoptListener("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		log.Printf("%+v", req)
		// The "/" pattern matches everything, so we need to check
		// that we're at the root here.
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		fmt.Fprintf(w, "hello io_uring!")
	})

	s := http.Server{
		Handler: mux,
		ConnState: func(conn net.Conn, state http.ConnState) {
			fmt.Printf("conn: %+v, state: %+v\n", conn, state)
		},
	}
	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
