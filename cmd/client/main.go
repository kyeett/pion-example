package main

import (
	"log"
	"net/url"

	"github.com/kyeett/pion-example/internal/gameclient"
)

func main() {
	u := url.URL{Scheme: "ws", Host: "localhost:5000"}
	c, _ := gameclient.New(u, "7777", func(msg []byte) error {
		log.Println("message received")
		return nil
	})
	c.Start()
}
