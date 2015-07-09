package cine

import (
	"log"
	"testing"
)

type Phonebook struct {
	Actor
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
	log.Println("Actor terminated:", errReason)
}

func TestDirector(t *testing.T) {
	Init("127.0.0.1:9000")
	book := Phonebook{Actor{}, make(map[string]int)}
	pid := StartActor(&book)
	defer Stop(pid)
	if pid.NodeName != "127.0.0.1:9000" {
		t.Errorf("pid.NodeName shoud be 127.0.0.1:9000 but was %v\n", pid.NodeName)
	}

	Cast(pid, make(chan *ActorCall, 1), (*Phonebook).Add, "Jane", 1234)
	r, err := Call(pid, (*Phonebook).Lookup, "Jane")
	if err != nil {
		t.Errorf("Expected no error but got %v\n", err)
	}
	if r[0].(int) != 1234 {
		t.Errorf("Expected 1234 return but got %v\n", r)
	}

	err = Stop(pid)
	if err != nil {
		t.Errorf("Expected no error, but got %v\n", err)
	}
}

func TestRemoteDirector(t *testing.T) {
	remoteD := NewDirector("127.0.0.1:9001")
	book := Phonebook{Actor{}, make(map[string]int)}
	pid := remoteD.StartActor(&book)
	defer remoteD.Stop(pid)
	if pid.String() != "<127.0.0.1:9001,1>" {
		t.Errorf("pid.NodeName shoud be 127.0.0.1:9001 but was %v\n", pid.NodeName)
	}

	remoteD.Call(pid, (*Phonebook).Add, "Jane", 1234)
	r, err := remoteD.Call(pid, (*Phonebook).Lookup, "Jane")
	if err != nil {
		t.Errorf("Expected no error but got %v\n", err)
	}
	if r[0].(int) != 1234 {
		t.Errorf("Expected 1234 return but got %v\n", r)
	}

	d := NewDirector("127.0.0.1:9002")
	d.Cast(pid, make(chan *ActorCall, 1), (*Phonebook).Add, "Jane", 2341)
	d.Cast(pid, make(chan *ActorCall, 1), (*Phonebook).Add, "Jane", 2342)
	d.Cast(pid, make(chan *ActorCall, 1), (*Phonebook).Add, "Jane", 2343)
	d.Cast(pid, make(chan *ActorCall, 1), (*Phonebook).Add, "Jane", 2344)
	d.Cast(pid, make(chan *ActorCall, 1), (*Phonebook).Add, "Jane", 2345)
	d.Cast(pid, make(chan *ActorCall, 1), (*Phonebook).Add, "Jane", 2346)

	r, err = d.Call(pid, (*Phonebook).Lookup, "Jane")
	if err == nil {
		t.Error("Expected call error, but got no error")
	}
}
