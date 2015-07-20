package main

import (
	"os"
	"sync"
	"time"

	"github.com/devsisters/cine"
	"github.com/golang/glog"
)

var waitGroup sync.WaitGroup

type PlayerProxy struct {
	Pid cine.Pid
}

func (p *PlayerProxy) Start(to cine.Pid) {
	cine.Call(p.Pid, (*Player).HandleStart, to)
}

func (p *PlayerProxy) Ping(sender cine.Pid, count int) {
	cine.Cast(p.Pid, nil, (*Player).HandlePing, sender, count)
}

func (p *PlayerProxy) Pong(sender cine.Pid, count int) {
	cine.Cast(p.Pid, nil, (*Player).HandlePong, sender, count)
}

type Player struct {
	cine.Actor
	count int
}

func (p *Player) HandleStart(to cine.Pid) {
	glog.Infoln("Start pingpong with", to)
	otherPlayer := PlayerProxy{Pid: to}
	otherPlayer.Ping(p.Self(), p.count)
}

func (p *Player) HandlePing(sender cine.Pid, count int) {
	glog.Infoln(sender, "Ping:", count)
	if p.count == 10 {
		glog.Infoln(sender, "Stopping game")
		cine.Stop(p.Self())
		return
	}
	time.Sleep(500 * time.Millisecond)
	p.count += 1

	otherPlayer := PlayerProxy{Pid: sender}
	otherPlayer.Pong(p.Self(), p.count)
}

func (p *Player) HandlePong(sender cine.Pid, count int) {
	glog.Infoln(sender, "Pong:", count)
	time.Sleep(500 * time.Millisecond)
	p.count += 1

	otherPlayer := PlayerProxy{Pid: sender}
	otherPlayer.Ping(p.Self(), p.count)

	if p.count == 10 {
		glog.Infoln(sender, "Stopping game")
		cine.Stop(p.Self())
		return
	}
}

func (p *Player) Terminate(errReason error) {
	glog.Infoln("Actor terminated:", errReason)
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
	glog.Infoln("pid:", pid)
	if playerNum == "2" {
		to := cine.Pid{"127.0.0.1:3000", 1}
		myPlayer := PlayerProxy{Pid: pid}
		myPlayer.Start(to)
	}
	waitGroup.Wait()
}
