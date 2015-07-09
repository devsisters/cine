package main

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/devsisters/cine"
)

var waitGroup sync.WaitGroup

type Player struct {
	cine.Actor
	count int
}

func (p *Player) Start(to cine.Pid) {
	log.Println("Start pingpong with", to)
	cine.Cast(to, make(chan *cine.ActorCall, 1), (*Player).Ping, p.Self(), p.count)
}

func (p *Player) Ping(sender cine.Pid, count int) {
	log.Println(sender, "Ping:", count)
	if p.count == 10 {
		log.Println(sender, "Stopping game")
		cine.Stop(p.Self())
		return
	}
	time.Sleep(500 * time.Millisecond)
	p.count += 1
	cine.Cast(sender, make(chan *cine.ActorCall, 1), (*Player).Pong, p.Self(), p.count)
}

func (p *Player) Pong(sender cine.Pid, count int) {
	log.Println(sender, "Pong:", count)
	time.Sleep(500 * time.Millisecond)
	p.count += 1
	cine.Cast(sender, make(chan *cine.ActorCall, 1), (*Player).Ping, p.Self(), p.count)
	if p.count == 10 {
		log.Println(sender, "Stopping game")
		cine.Stop(p.Self())
		return
	}
}

func (p *Player) Terminate(errReason error) {
	log.Println("Actor terminated:", errReason)
	waitGroup.Done()
}

func main() {
	playerNum := os.Args[1]
	if playerNum == "1" {
		cine.Init("127.0.0.1:3000")
	} else {
		cine.Init("127.0.0.1:3001")
	}
	player := Player{cine.Actor{}, 0}
	waitGroup.Add(1)
	pid := cine.StartActor(&player)
	log.Println("pid:", pid)
	if playerNum == "2" {
		to := cine.Pid{"127.0.0.1:3000", 1}
		cine.Call(pid, (*Player).Start, to)
	}
	waitGroup.Wait()
}
