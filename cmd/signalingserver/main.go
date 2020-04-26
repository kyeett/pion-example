package main

import (
	"github.com/kyeett/pion-example/internal/signalingserver"
	"log"
)

func main() {
	s := signalingserver.New()
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
}


