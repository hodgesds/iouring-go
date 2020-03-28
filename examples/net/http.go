package main

import (
	"flag"
	"fmt"
	"http"
	"log"
	"net"
)

var port int

func init() {
	flag.IntVar(&port, "port", 9870, "HTTP port")
}

func main() {
	// Steps:
	// 1) create socket
	// 2) set sockopt
	// 3) bind to port
	// 4) listen
	// 5) add first io_uring poll sqe, to check when there will be data available on sock_listen_fd
	flag.Parse()
	l, err := net.ListenIP("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	s := &http.Server{}
	s.Listen(l)
}
