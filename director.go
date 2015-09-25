package cine

import (
	"encoding/gob"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
)

var DefaultDirector *Director

func init() {
	gob.Register(Pid{})
}

func Init(nodeName string) {
	// TODO(serialx): Thread-safe init?
	if DefaultDirector == nil {
		DefaultDirector = NewDirector(nodeName)
	}
}

// Ask the kernel for a free open port that is ready to use
func getPort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func InitForTest() {
	if DefaultDirector == nil {
		DefaultDirector = NewDirector(fmt.Sprintf("127.0.0.1:%d", getPort()))
	}
}

type DirectorError struct {
	Message string
}

func StartActor(actorImpl ActorImplementor) Pid {
	if DefaultDirector == nil {
		panic("DefaultDirector not initialized. Call cine.Init first.")
	}
	return DefaultDirector.StartActor(actorImpl)
}

func Call(pid Pid, function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	if DefaultDirector == nil {
		panic("DefaultDirector not initialized. Call cine.Init first.")
	}
	return DefaultDirector.Call(pid, function, args...)
}

func CallWithContext(pid Pid, function interface{}, ctx context.Context, args ...interface{}) ([]interface{}, *DirectorError) {
	if DefaultDirector == nil {
		panic("DefaultDirector not initialized. Call cine.Init first.")
	}
	return DefaultDirector.CallWithContext(pid, function, ctx, args...)
}

func Cast(pid Pid, done chan *ActorCall, function interface{}, args ...interface{}) {
	if DefaultDirector == nil {
		panic("DefaultDirector not initialized. Call cine.Init first.")
	}
	DefaultDirector.Cast(pid, done, function, args...)
}

func Stop(pid Pid) *DirectorError {
	if DefaultDirector == nil {
		panic("DefaultDirector not initialized. Call cine.Init first.")
	}
	return DefaultDirector.Stop(pid)
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
	cast(done chan *ActorCall, function interface{}, args ...interface{})
	callWithContext(function interface{}, ctx context.Context, args ...interface{}) ([]interface{}, *DirectorError)
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
	clientLock sync.Mutex
	clientMap  map[string]*rpc.Client
	maxActorId int
	server     *http.Server
}

func NewDirector(nodeName string) *Director {
	d := &Director{
		nodeName:   nodeName,
		pidMap:     make(map[Pid]*Actor),
		clientMap:  make(map[string]*rpc.Client),
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

	_, port, err := net.SplitHostPort(d.nodeName)
	if err != nil {
		panic(err)
	}

	d.server = &http.Server{
		Addr:           ":" + port,
		Handler:        rpc,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		log.Infoln("Director listening at", d.nodeName)
		// Partially ensure the director is up before returning
		wg.Done()
		log.Fatalln(d.server.ListenAndServe())
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

func (d *Director) removeClient(pid Pid) {
	d.clientLock.Lock()
	defer d.clientLock.Unlock()

	delete(d.clientMap, pid.NodeName)
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
	d.clientLock.Lock()
	client, ok := d.clientMap[pid.NodeName]
	d.clientLock.Unlock()
	// It's okay that we override client due to race condition
	if !ok {
		var err error
		client, err = rpc.DialHTTP("tcp", pid.NodeName)
		if err != nil {
			return nil, err
		}
		// TODO(serialx): We don't cache the client connections for now.
		// The reason is that client connection closes after the ReadTimeout/
		// WriteTimeout	which is currently 10 seconds. We need to manage that
		// before we can reliably cache the client connections.
		//d.clientLock.Lock()
		//d.clientMap[pid.NodeName] = client
		//d.clientLock.Unlock()
	}
	rActor := &RemoteActor{
		pid:      pid,
		client:   client,
		director: d,
	}
	return rActor, nil
}

func (d *Director) actorFromPid(pid Pid) (actorLike, error) {
	if pid.NodeName != d.nodeName {
		return d.remoteActorFromPid(pid)
	}
	return d.localActorFromPid(pid)
}

// Call method calls the function on the pid actors goroutine.
// ErrActorNotFound can be returned if pid does not exist or remote node is
// unavailable.
func (d *Director) Call(pid Pid, function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	actor, err := d.actorFromPid(pid)
	if err != nil {
		return nil, ErrActorNotFound
	}
	return actor.call(function, args...)
}

func (d *Director) Cast(pid Pid, done chan *ActorCall, function interface{}, args ...interface{}) {
	actor, err := d.actorFromPid(pid)
	if err != nil {
		return
	}
	actor.cast(done, function, args...)
}

func (d *Director) CallWithContext(pid Pid, function interface{}, ctx context.Context, args ...interface{}) ([]interface{}, *DirectorError) {
	actor, err := d.actorFromPid(pid)
	if err != nil {
		return nil, ErrActorNotFound
	}

	type Return struct {
		ret []interface{}
		err *DirectorError
	}
	c := make(chan Return, 1)
	go func() { ret, err := actor.callWithContext(function, ctx, args...); c <- Return{ret: ret, err: err} }()
	select {
	case <-ctx.Done():
		return nil, &DirectorError{ctx.Err().Error()}
	case ret := <-c:
		return ret.ret, ret.err
	}

	return actor.callWithContext(function, ctx, args...)
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
	Timeout      string
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

func (d *DirectorApi) HandleRemoteCallWithContext(r RemoteRequest, reply *RemoteResponse) error {
	fun, err := d.findFun(r)
	if err != nil {
		reply.Err = err
		return nil
	}
	// construct context
	timeout, parseErr := time.ParseDuration(r.Timeout)
	if parseErr != nil {
		reply.Err = &DirectorError{parseErr.Error()}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ret, err := d.director.CallWithContext(r.Pid, fun, ctx, r.Args...)
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
	done := make(chan *ActorCall, 1)
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
