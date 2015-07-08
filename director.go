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
	ErrActorDied      = &DirectorError{"Actor died"}
	ErrActorNotFound  = &DirectorError{"Actor not found"}
	ErrMethodNotFound = &DirectorError{"Method not found"}
	ErrActorStop      = &DirectorError{"Actor stop"}
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
	call(function interface{}, args ...interface{}) ([]interface{}, *DirectorError)
	cast(done chan *Call, function interface{}, args ...interface{})
	stop() *DirectorError
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
	rPidLock   sync.RWMutex
	rPidMap    map[Pid]*RemoteActor
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

func (d *Director) localActorFromPid(pid Pid) (*Actor, error) {
	d.pidLock.RLock()
	defer d.pidLock.RUnlock()

	actor, ok := d.pidMap[pid]
	if !ok {
		return nil, ErrActorNotFound
	}
	return actor, nil
}

func (d *Director) remoteActorFromPid(pid Pid) (*RemoteActor, error) {
	d.rPidLock.RLock()
	defer d.rPidLock.RUnlock()

	rActor, ok := d.rPidMap[pid]
	if !ok {
		rActor = &RemoteActor{pid: pid}
		rActor.actor.startMessageLoop(rActor)
		return rActor, nil
	}
	return rActor, nil
}

func (d *Director) actorFromPid(pid Pid) (actorLike, error) {
	if pid.NodeName != d.nodeName {
		return d.remoteActorFromPid(pid)
	}
	return d.localActorFromPid(pid)
}

func (d *Director) Call(pid Pid, function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	actor, err := d.actorFromPid(pid)
	if err != nil {
		return nil, ErrActorNotFound
	}
	return actor.call(function, args...)
}

func (d *Director) Cast(pid Pid, done chan *Call, function interface{}, args ...interface{}) {
	actor, err := d.actorFromPid(pid)
	if err != nil {
		return
	}
	actor.cast(done, function, args...)
}

func (d *Director) Stop(pid Pid) *DirectorError {
	actor, err := d.actorFromPid(pid)
	if err != nil {
		return ErrActorNotFound
	}
	actor.stop()
	return nil
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
	actor, err := d.director.localActorFromPid(r.Pid)
	if err != nil {
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
	fun, err := d.findFun(r)
	if err != nil {
		reply.Err = err
		return nil
	}
	ret, err := d.director.Call(r.Pid, fun, r.Args...)
	if err != nil {
		reply.Err = err
		return nil
	}
	reply.Return = ret
	return nil
}

func (d *DirectorApi) HandleRemoteCast(r RemoteRequest, reply *RemoteResponse) error {
	fun, err := d.findFun(r)
	if err != nil {
		reply.Err = err
		return nil
	}
	done := make(chan *Call, 1)
	d.director.Cast(r.Pid, done, fun, r.Args...)
	return nil
}

func (d *DirectorApi) HandleRemoteStop(r RemoteRequest, reply *RemoteResponse) error {
	err := d.director.Stop(r.Pid)
	if err != nil {
		reply.Err = err
	}
	return nil
}
