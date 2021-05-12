package main

import (
	"log"

	"github.com/jinzhu/copier"
)

type m struct {
	mm map[int]bool
}

func insert(m map[string]bool) {
	m["b"] = false
}

func sliceInsert(s []int) {
	s[0] = 0
}

func main() {
	/*
		c := multicast.New()

		go func() {
			l := c.Listen()
			for msg := range l.C {
				fmt.Printf("Listener 1: %s\n", msg)
			}
			fmt.Println("Listener 1 Closed")
		}()
			go func() {
				l := c.Listen()
				for msg := range l.C {
					fmt.Printf("Listener 2: %s\n", msg)
				}
				fmt.Println("Listener 2 Closed")
			}()

		c.C <- "Hello World!"
		for {
			time.Sleep(1 * time.Second)
		}
	*/

	a := make(map[string]bool)
	a["a"] = true
	insert(a)
	delete(a, "a")
	log.Println(a)

	b := []int{1, 2}
	sliceInsert(b)
	log.Println(b)

	var c []int
	c = append(c, 0)
	log.Println(c)

	mp1 := m{
		mm: map[int]bool{1: true},
	}

	mp2 := m{
		mm: make(map[int]bool),
	}

	copier.Copy(&mp2, &mp1)
	mp2.mm[1] = false

	// Shallow copy
	log.Println(mp1)
	log.Println(mp2)
}
