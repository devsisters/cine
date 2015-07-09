package main

import (
	"fmt"

	"github.com/devsisters/cine"
)

type Phonebook struct {
	cine.Actor
	book map[string]int
}

func (b *Phonebook) Add(name string, number int) {
	if number == 2344 {
		panic("haha panic!")
	}
	b.book[name] = number
}

func (b *Phonebook) Lookup(name string) (int, bool) {
	a, ok := b.book[name]
	return a, ok
}

func (b *Phonebook) Terminate(errReason error) {
	fmt.Println("Actor terminated:", errReason)
}

func main() {
	cine.Init("127.0.0.1:9000")
	phonebook := Phonebook{cine.Actor{}, make(map[string]int)}
	pid := cine.StartActor(&phonebook)
	fmt.Println("pid:", pid)
	cine.Call(pid, (*Phonebook).Add, "Jane", 1234)
	r, _ := cine.Call(pid, (*Phonebook).Lookup, "Jane")
	fmt.Printf("Lookup('Jane') == %v\n", r[0].(int))
}
