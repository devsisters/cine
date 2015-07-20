package main

import (
	"github.com/devsisters/cine"
	"github.com/golang/glog"
)

type Phonebook struct {
	cine.Actor
	book map[string]int
}

func (p *Phonebook) Add(name string, number int) {
	p.book[name] = number
}

func (p *Phonebook) Lookup(name string) (int, bool) {
	result, ok := p.book[name]
	return result, ok
}

func (p *Phonebook) Terminate(errReason error) {
}

func main() {
	cine.Init("127.0.0.1:8000")
	phonebook := Phonebook{cine.Actor{}, make(map[string]int)}
	pid := cine.StartActor(&phonebook)

	cine.Cast(pid, nil, (*Phonebook).Add, "Jane", 1234)
	ret, _ := cine.Call(pid, (*Phonebook).Lookup, "Jane")
	number := ret[0].(int)
	glog.Infoln("number:", number)
	// Out: number: 1234
}
