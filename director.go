package cinema

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/rpc"
	"sync"
	"time"
)

var ErrActorNotFound = errors.New("Actor not found")
var ErrMethodNotFound = errors.New("Method not found")
var ErrShutdown = errors.New("Actor shutdown")

type PanicError struct {
	PanicErr interface{}
}

func (e PanicError) Error() string {
	return fmt.Sprintf("Actor panic: %v", e.PanicErr)
}

type ActorImplementor interface {
	ActorLike
	startMessageLoop(receiver interface{})
	GetActor() *Actor
	Terminate(errReason error)
}

type ActorLike interface {
	Call(function interface{}, args ...interface{}) []interface{}
	Cast(out chan<- Response, function interface{}, args ...interface{})
}

type Pid struct {
	NodeName string
	ActorId  int
}

type Director struct {
	nodeName   string
	pidLock    sync.RWMutex
	pidMap     map[Pid]*Actor
	maxActorId int
	server     *http.Server
}

func NewDirector(nodeName string) *Director {
	d := &Director{
		nodeName:   nodeName,
		pidMap:     make(map[Pid]*Actor),
		maxActorId: 0,
	}
	d.startServer()
	return d
}

func (d *Director) startServer() {
	rpc := rpc.NewServer()
	directorApi := &DirectorApi{
		director: d,
	}
	rpc.Register(directorApi)
	d.server = &http.Server{
		Addr:           d.nodeName,
		Handler:        rpc,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Println("Director listening at", d.nodeName)
		// Partially ensure the director is up before returning
		wg.Done()
		log.Fatal(d.server.ListenAndServe())
	}()
	wg.Wait()
}

// createPid must be called within d.pidLock critical section
func (d *Director) createPid() Pid {
	d.maxActorId += 1
	return Pid{d.nodeName, d.maxActorId}
}

func (d *Director) StartActor(actorImpl ActorImplementor) Pid {
	actor := actorImpl.GetActor()
	actorImpl.startMessageLoop(actorImpl)
	d.pidLock.Lock()
	defer d.pidLock.Unlock()
	pid := d.createPid()
	actor.pid = pid
	actor.director = d
	d.pidMap[pid] = actor
	return pid
}

func (d *Director) removeActor(pid Pid) {
	d.pidLock.Lock()
	defer d.pidLock.Unlock()

	delete(d.pidMap, pid)
}

func (d *Director) localActorFromPid(pid Pid) *Actor {
	d.pidLock.RLock()
	defer d.pidLock.RUnlock()

	actor, ok := d.pidMap[pid]
	if !ok {
		return nil
	}
	return actor
}

func (d *Director) remoteActorFromPid(pid Pid) *RemoteActor {
	return &RemoteActor{pid: pid}
}

func (d *Director) actorFromPid(pid Pid) ActorLike {
	if pid.NodeName != d.nodeName {
		return d.remoteActorFromPid(pid)
	}
	return d.localActorFromPid(pid)
}

func (d *Director) Call(pid Pid, function interface{}, args ...interface{}) []interface{} {
	actor := d.actorFromPid(pid)
	return actor.Call(function, args...)
}

func (d *Director) Cast(pid Pid, out chan<- Response, function interface{}, args ...interface{}) {
	actor := d.actorFromPid(pid)
	actor.Cast(out, function, args...)
}

type DirectorApi struct {
	director *Director
}

type RemoteRequest struct {
	Pid          Pid
	FunctionName string
	Args         []interface{}
}

type RemoteResponse struct {
	Err    error
	Return []interface{}
}

func (d *DirectorApi) findFun(r RemoteRequest) (interface{}, error) {
	actor := d.director.localActorFromPid(r.Pid)
	if actor == nil {
		return nil, ErrActorNotFound
	}

	t := actor.receiver.Type()
	method, ok := t.MethodByName(r.FunctionName)
	if !ok {
		return nil, ErrMethodNotFound
	}
	fun := method.Func.Interface()
	return fun, nil
}

func (d *DirectorApi) HandleRemoteCall(r RemoteRequest, reply *RemoteResponse) error {
	log.Println("HandleRemoteCall", r)
	fun, err := d.findFun(r)
	if err != nil {
		reply.Err = err
		return nil
	}
	ret := d.director.Call(r.Pid, fun, r.Args...)
	reply.Return = ret
	return nil
}

func (d *DirectorApi) HandleRemoteCast(r RemoteRequest, reply *RemoteResponse) error {
	log.Println("HandleRemoteCast", r)
	fun, err := d.findFun(r)
	if err != nil {
		reply.Err = err
		return nil
	}
	out := make(chan Response, 1)
	d.director.Cast(r.Pid, out, fun, r.Args...)
	return nil
}
