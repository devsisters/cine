package cine

import (
	"fmt"
	"reflect"
	"sync"

	"golang.org/x/net/context"

	"github.com/go-errors/errors"
	log "github.com/Sirupsen/logrus"
)

type Actor struct {
	pid      Pid
	director *Director
	queue    *MessageQueue
	receiver reflect.Value

	// alive status should be protected with mutex to create memory barrier
	// because methods like call(), stop() will be called in another thread
	aliveLock sync.Mutex
	alive     bool

	shutdownCh chan bool
}

const kActorQueueLength int = 1

func (r *Actor) Self() Pid {
	return r.pid
}

// call method synchronously calls function in the actor's thread.
func (r *Actor) call(function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	r.aliveLock.Lock()
	if !r.alive {
		r.aliveLock.Unlock()
		return nil, ErrActorStop
	}
	r.aliveLock.Unlock()

	done := make(chan *ActorCall, 0)
	r.cast(done, function, args...)
	response, ok := <-done
	if !ok {
		return nil, ErrActorDied
	}

	return response.ReplyAsInterfaces(), nil
}

// callWithContext function make an assumption that receive function's first argument is context
func (r *Actor) callWithContext(function interface{}, ctx context.Context, args ...interface{}) ([]interface{}, *DirectorError) {
	args = append([]interface{}{ctx}, args...)
	return r.call(function, args...)
}

// getActor used by Director
func (r *Actor) getActor() *Actor {
	return r
}

// verifyCallSignature confirms whether the function is callable on the receiver
func (r *Actor) verifyCallSignature(function interface{}, args []interface{}) {
	typ := reflect.TypeOf(function)
	if typ == nil {
		panic("Function is nil")
	}
	if typ.Kind() != reflect.Func {
		panic("Function is not a method")
	}
	if typ.NumIn() < 1 {
		panic("Function is not a method. Function has no receiver")
	}
	if !r.receiver.Type().AssignableTo(typ.In(0)) {
		panic(fmt.Sprintf(
			"Cannot assign receiver (of type %s) to %s", r.receiver.Type(), typ.In(0)))
	}
	numNonReceiver := typ.NumIn() - 1
	if len(args) < numNonReceiver {
		panic(fmt.Sprintf(
			"Not enough arguments given (needed %d, got %d)", numNonReceiver, len(args)))
	}
	if len(args) > numNonReceiver && !typ.IsVariadic() {
		panic(fmt.Sprintf("Too many args for non-variadic function (needed %d, got %d)",
			numNonReceiver, len(args)))
	}
	for i := 1; i < typ.NumIn(); i++ {
		if argType := reflect.TypeOf(args[i-1]); !argType.AssignableTo(typ.In(i)) {
			panic(
				fmt.Sprintf("Cannot assign arg %d (%s -> %s)", i-1, argType, typ.In(i)))
		}
	}
}

// cast method asynchronously calls function in the actor's thread. This function does
// not return anything. Errors or panic caused by the function is not passed to the
// caller.
func (r *Actor) cast(done chan *ActorCall, function interface{}, args ...interface{}) {
	r.aliveLock.Lock()
	if !r.alive {
		r.aliveLock.Unlock()
		return
	}
	r.aliveLock.Unlock()

	r.verifyCallSignature(function, args)
	r.runInThread(done, r.receiver, function, args...)
}

func (r *Actor) runInThread(done chan *ActorCall, receiver reflect.Value, function interface{}, args ...interface{}) {
	if r.queue == nil {
		panic("Call startMessageLoop before sending it messages!")
	}

	// reflect.Call expects the arguments to be a slice of reflect.Values. We also
	// need to ensure that the 0th argument is the receiving struct.
	valuedArgs := make([]reflect.Value, len(args)+1)
	valuedArgs[0] = receiver
	for i, x := range args {
		valuedArgs[i+1] = reflect.ValueOf(x)
	}

	r.queue.In <- &ActorCall{reflect.ValueOf(function), valuedArgs, nil, done}
}

func (r *Actor) processOneRequest(request *ActorCall) {
	request.Reply = request.Function.Call(request.Args)
	if request.Done != nil {
		request.Done <- request
	}
}

// terminateActor terminates the actor. Should be only called within actor thread
func (r *Actor) terminateActor(errReason error) {
	if r.director != nil {
		r.director.removeActor(r.pid)
	}

	r.aliveLock.Lock()
	r.alive = false
	r.queue.Stop <- true
	r.aliveLock.Unlock()

	r.receiver.Interface().(ActorImplementor).Terminate(errReason)
}

func (r *Actor) messageLoop() {
	var lastCall *ActorCall
	defer func() {
		if e := recover(); e != nil {
			// XXX(serialx): It's weird. The stacktrace is not properly rendered.

			// Actor panicked
			panicErr := errors.Wrap(e, 2)
			errPanic := &PanicError{PanicErr: panicErr}

			stacktrace := panicErr.ErrorStack()
			log.Errorf("actor panic: %s\n", stacktrace)

			r.terminateActor(errPanic)
			if lastCall != nil {
				close(lastCall.Done)
			}
		}
	}()

ForLoop:
	for {
		select {
		case call, ok := <-r.queue.Out:
			if !ok {
				break ForLoop
			}
			lastCall = call
			r.processOneRequest(call)
		case <-r.shutdownCh:
			r.terminateActor(ErrActorStop)
		}
	}
}

// startMessageLoop starts the actor thread.
// This must be called before any actor calls and casts.
func (r *Actor) startMessageLoop(receiver interface{}) {
	r.queue = NewMessageQueue(kActorQueueLength)
	r.receiver = reflect.ValueOf(receiver)
	// Make this buffered so the actor can self stop
	r.shutdownCh = make(chan bool, 1)

	r.aliveLock.Lock()
	r.alive = true
	r.aliveLock.Unlock()

	go r.messageLoop()
}

// stop stops the actor thread.
func (r *Actor) stop() *DirectorError {
	r.aliveLock.Lock()
	defer r.aliveLock.Unlock()
	if r.alive {
		// Pass nil function pointer to stop the message loop
		r.alive = false
		r.shutdownCh <- true
	}
	return nil
}
