Cine
====
[![Circle CI](https://circleci.com/gh/devsisters/cine.svg?style=svg)](https://circleci.com/gh/devsisters/cine)

Actor model for golang.

This project aims to implement actor model from erlang to Go. It attempts to
add a concept of `Pid` which is universal identifier for actors.

Actor supports `Call` which is a syncronous method call. `Cast` which is
asyncronous method call ignoring all errors.

Actors can communicate with other actors using `Call` and `Cast`. Remote actors
are treated same as local actors.

This project is originally based on [GLAM](https://github.com/areusch/glam)

Usage
=====

```go
package main

import (
	log "github.com/Sirupsen/logrus"

	"github.com/devsisters/cine"
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
	log.Infoln("number:", number)
	// Out: number: 1234
}
```

Performance
===========

Preliminary benchmarks indicate about 5x overhead over vanilla channels. Do not
use actors for call heavy operations.

```
BenchmarkChannel	3000000	       583 ns/op
BenchmarkActor	  	500000	      2850 ns/op
```

