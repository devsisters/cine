package main

import (
	"fmt"

	"github.com/devsisters/cinema"
)

type Phonebook struct {
	cinema.Actor
	book map[string]int
}

func (b *Phonebook) Add(name string, number int) {
	/*
		if number == 2345 {
			fmt.Println("panic!")
			panic("haha panic!")
		}
	*/
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
	//book := Phonebook{cinema.Actor{}, make(map[string]int)}
	//book.StartActor(&book)
	//book.Call((*Phonebook).Add, "Jane", 1234)
	//r := book.Call((*Phonebook).Lookup, "Jane")[0].(int)
	//fmt.Printf("Lookup('Jane') == %v\n", r)
	//book.StopActor()

	/*
		d := cinema.NewDirector("127.0.0.1:8080")
		book := Phonebook{cinema.Actor{}, make(map[string]int)}
		pid := d.StartActor(&book)
		fmt.Println("pid:", pid)
		d.Call(pid, (*Phonebook).Add, "Jane", 1234)
		r := d.Call(pid, (*Phonebook).Lookup, "Jane")[0].(int)
		fmt.Printf("Lookup('Jane') == %v\n", r)
	*/

	remoteD := cinema.NewDirector("127.0.0.1:9000")
	book := Phonebook{cinema.Actor{}, make(map[string]int)}
	pid := remoteD.StartActor(&book)
	fmt.Println("pid:", pid)
	remoteD.Call(pid, (*Phonebook).Add, "Jane", 1234)
	r := remoteD.Call(pid, (*Phonebook).Lookup, "Jane")[0].(int)
	fmt.Printf("Lookup('Jane') == %v\n", r)

	d := cinema.NewDirector("127.0.0.1:8080")
	out := make(chan cinema.Response, 1)
	d.Cast(pid, out, (*Phonebook).Add, "Jane", 2341)
	out = make(chan cinema.Response, 1)
	d.Cast(pid, out, (*Phonebook).Add, "Jane", 2342)
	out = make(chan cinema.Response, 1)
	d.Cast(pid, out, (*Phonebook).Add, "Jane", 2343)
	out = make(chan cinema.Response, 1)
	d.Cast(pid, out, (*Phonebook).Add, "Jane", 2344)
	out = make(chan cinema.Response, 1)
	d.Cast(pid, out, (*Phonebook).Add, "Jane", 2345)
	out = make(chan cinema.Response, 1)
	d.Cast(pid, out, (*Phonebook).Add, "Jane", 2346)

	r = d.Call(pid, (*Phonebook).Lookup, "Jane")[0].(int)
	fmt.Printf("(remote) Lookup('Jane') == %v\n", r)
}
