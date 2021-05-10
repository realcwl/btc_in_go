package main

import (
	"fmt"
	"time"

	"github.com/SierraSoftworks/multicast"
)

func main() {
	c := multicast.New()

	go func() {
		l := c.Listen()
		for msg := range l.C {
			fmt.Printf("Listener 1: %s\n", msg)
		}
		fmt.Println("Listener 1 Closed")
	}()
	/*
		go func() {
			l := c.Listen()
			for msg := range l.C {
				fmt.Printf("Listener 2: %s\n", msg)
			}
			fmt.Println("Listener 2 Closed")
		}()
	*/
	c.C <- "Hello World!"
	for {
		time.Sleep(1 * time.Second)
	}
}
