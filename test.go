package main

import "log"

type Inf interface {
	Say() string
	Set(s string)
}

type InfImpl struct {
	Inf
}

func (*InfImpl) Say() string {
	return "hello world"
}

func main() {
	i := InfImpl{}
	log.Println(i.Say())
}
