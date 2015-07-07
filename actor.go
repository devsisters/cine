package cinema

import (
	"fmt"
	"reflect"
	"sync"
)

type Actor struct {
	pid      Pid
	director *Director
	queue    *MessageQueue
	receiver reflect.Value
	current  chan<- Response

	// alive status should be protected with mutex to create memory barrier
	// because methods like call(), stop() will be called in another thread
	alive     bool
	aliveLock sync.Mutex
}

const kActorQueueLength int = 1

// call method synchronously calls function in the actor's thread.
func (r *Actor) call(function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	r.aliveLock.Lock()
	if !r.alive {
		r.aliveLock.Unlock()
		return nil, ErrActorStop
	}
	r.aliveLock.Unlock()

	out := make(chan Response, 0)
	r.cast(out, function, args...)
	response, ok := <-out
	if !ok {
		return nil, ErrActorDied
	}

	return response.InterpretAsInterfaces(), nil
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
func (r *Actor) cast(out chan<- Response, function interface{}, args ...interface{}) {
	r.aliveLock.Lock()
	if !r.alive {
		r.aliveLock.Unlock()
		return
	}
	r.aliveLock.Unlock()

	r.verifyCallSignature(function, args)
	r.runInThread(out, r.receiver, function, args...)
}

func (r *Actor) runInThread(out chan<- Response, receiver reflect.Value, function interface{}, args ...interface{}) {
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

	r.queue.In <- Request{reflect.ValueOf(function), valuedArgs, out}
}

func (r *Actor) processOneRequest(request Request) {
	r.current = request.ReplyTo
	result := request.Function.Call(request.Args)
	response := ResponseImpl{result: result, err: nil, panicked: false}
	if request.ReplyTo != nil {
		request.ReplyTo <- response
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

// startMessageLoop starts the actor thread.
// This must be called before any actor calls and casts.
func (r *Actor) startMessageLoop(receiver interface{}) {
	r.queue = NewMessageQueue(kActorQueueLength)
	r.receiver = reflect.ValueOf(receiver)

	r.aliveLock.Lock()
	r.alive = true
	r.aliveLock.Unlock()

	go func() {
		var lastReq *Request
		defer func() {
			if e := recover(); e != nil {
				// Actor panicked
				errPanic := &PanicError{PanicErr: e}
				r.terminateActor(errPanic)
				if lastReq != nil {
					close(lastReq.ReplyTo)
				}
			}
		}()

		for {
			request, ok := <-r.queue.Out
			if !ok {
				// The queue is stopped. We should terminate
				r.terminateActor(ErrActorStop)
				break
			}
			lastReq = &request
			r.processOneRequest(request)
		}
	}()
}

// stop stops the actor thread.
func (r *Actor) stop() *DirectorError {
	r.aliveLock.Lock()
	defer r.aliveLock.Unlock()
	if r.alive {
		// Pass nil function pointer to stop the message loop
		r.queue.In <- Request{reflect.ValueOf((func())(nil)), nil, nil}
		r.alive = false
	}
	return nil
}
