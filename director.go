package cinema

import (
	"fmt"
	"log"
	"net/http"
	"net/rpc"
	"sync"
	"time"
)

type DirectorError struct {
	Message string
}

func (e *DirectorError) Error() string {
	return e.Message
}

var (
	ErrActorNotFound  = &DirectorError{"Actor not found"}
	ErrMethodNotFound = &DirectorError{"Method not found"}
	ErrShutdown       = &DirectorError{"Actor shutdown"}
)

type PanicError struct {
	PanicErr interface{}
}

func (e PanicError) Error() string {
	return fmt.Sprintf("Actor panic: %v", e.PanicErr)
}

type ActorImplementor interface {
	actorLike
	Terminate(errReason error)

	startMessageLoop(receiver interface{})
	getActor() *Actor
}

type actorLike interface {
	call(function interface{}, args ...interface{}) ([]interface{}, error)
	cast(out chan<- Response, function interface{}, args ...interface{})
}

type Pid struct {
	NodeName string
	ActorId  int
}

func (p Pid) String() string {
	return fmt.Sprintf("<%s,%d>", p.NodeName, p.ActorId)
}

type Director struct {
	nodeName   string
	pidLock    sync.RWMutex
	pidMap     map[Pid]*Actor
	maxActorId int
	server     *http.Server
}

/*
func init() {
	gob.Register(DirectorError{})
}
*/

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
	actor := actorImpl.getActor()
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

func (d *Director) actorFromPid(pid Pid) actorLike {
	if pid.NodeName != d.nodeName {
		return d.remoteActorFromPid(pid)
	}
	return d.localActorFromPid(pid)
}

func (d *Director) Call(pid Pid, function interface{}, args ...interface{}) ([]interface{}, error) {
	actor := d.actorFromPid(pid)
	return actor.call(function, args...)
}

func (d *Director) Cast(pid Pid, out chan<- Response, function interface{}, args ...interface{}) {
	actor := d.actorFromPid(pid)
	actor.cast(out, function, args...)
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
	Err    *DirectorError
	Return []interface{}
}

func (d *DirectorApi) findFun(r RemoteRequest) (interface{}, *DirectorError) {
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
	ret, err_ := d.director.Call(r.Pid, fun, r.Args...)
	if err_ != nil {
		// Local call should never return an error
		panic("Call returned error")
	}
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
