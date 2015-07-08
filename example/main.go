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
	//book := Phonebook{cine.Actor{}, make(map[string]int)}
	//book.StartActor(&book)
	//book.Call((*Phonebook).Add, "Jane", 1234)
	//r := book.Call((*Phonebook).Lookup, "Jane")[0].(int)
	//fmt.Printf("Lookup('Jane') == %v\n", r)
	//book.StopActor()

	/*
		d := cine.NewDirector("127.0.0.1:8080")
		book := Phonebook{cine.Actor{}, make(map[string]int)}
		pid := d.StartActor(&book)
		fmt.Println("pid:", pid)
		d.Call(pid, (*Phonebook).Add, "Jane", 1234)
		r := d.Call(pid, (*Phonebook).Lookup, "Jane")[0].(int)
		fmt.Printf("Lookup('Jane') == %v\n", r)
	*/

	remoteD := cine.NewDirector("127.0.0.1:9000")
	book := Phonebook{cine.Actor{}, make(map[string]int)}
	pid := remoteD.StartActor(&book)
	fmt.Println("pid:", pid)
	remoteD.Call(pid, (*Phonebook).Add, "Jane", 1234)
	r, err := remoteD.Call(pid, (*Phonebook).Lookup, "Jane")
	fmt.Printf("Lookup('Jane') == %v\n", r[0].(int))

	d := cine.NewDirector("127.0.0.1:8080")
	d.Cast(pid, make(chan *cine.Call, 1), (*Phonebook).Add, "Jane", 2341)
	d.Cast(pid, make(chan *cine.Call, 1), (*Phonebook).Add, "Jane", 2342)
	d.Cast(pid, make(chan *cine.Call, 1), (*Phonebook).Add, "Jane", 2343)
	d.Cast(pid, make(chan *cine.Call, 1), (*Phonebook).Add, "Jane", 2344)
	d.Cast(pid, make(chan *cine.Call, 1), (*Phonebook).Add, "Jane", 2345)
	d.Cast(pid, make(chan *cine.Call, 1), (*Phonebook).Add, "Jane", 2346)

	r, err = d.Call(pid, (*Phonebook).Lookup, "Jane")
	if err != nil {
		fmt.Println("d.Call error:", err)
	} else {
		fmt.Printf("(remote) Lookup('Jane') == %v\n", r[0].(int))
	}

	/*
		d := cine.NewDirector("127.0.0.1:8080")
		book := Phonebook{cine.Actor{}, make(map[string]int)}
		pid := d.StartActor(&book)
		done := make(chan *cine.Call, 1)
		d.Cast(pid, done, (*Phonebook).Add, "Jane", 2341)
		done = make(chan *cine.Call, 1)
		d.Cast(pid, done, (*Phonebook).Add, "Jane", 2342)
		done = make(chan *cine.Call, 1)
		d.Cast(pid, done, (*Phonebook).Add, "Jane", 2343)
		done = make(chan *cine.Call, 1)
		d.Cast(pid, done, (*Phonebook).Add, "Jane", 2344)
		done = make(chan *cine.Call, 1)
		d.Cast(pid, done, (*Phonebook).Add, "Jane", 2345)
		done = make(chan *cine.Call, 1)
		d.Cast(pid, done, (*Phonebook).Add, "Jane", 2346)

		r, err := d.Call(pid, (*Phonebook).Lookup, "Jane")
		fmt.Println("r:", r, "err:", err)
		if err != nil {
			fmt.Println("d.Call error:", err)
		} else {
			fmt.Printf("(remote) Lookup('Jane') == %v\n", r[0].(int))
		}
	*/
}
