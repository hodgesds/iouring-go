package main

import (
	"log"

	"github.com/hodgesds/iouring-go"
)

func main() {
	r, err := iouring.New(2048)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Setup ring %v\n", r)
}
