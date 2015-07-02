package cinema

import (
	"fmt"
	"reflect"
)

// Internal state needed by the Actor.
type Actor struct {
	pid      Pid
	director *Director
	queue    *MessageQueue
	receiver reflect.Value
	current  chan<- Response
}

const kActorQueueLength int = 1

// Synchronously invoke function in the actor's own thread, passing args. Returns the
// result of execution.
func (r *Actor) call(function interface{}, args ...interface{}) ([]interface{}, *DirectorError) {
	out := make(chan Response, 0)
	r.cast(out, function, args...)
	response, ok := <-out
	if !ok {
		return nil, ErrActorDied
	}

	return response.InterpretAsInterfaces(), nil
}

func (r *Actor) getActor() *Actor {
	return r
}

// Internal method to verify that the given function can be invoked on the actor's
// receiver with the given args.
func (r *Actor) verifyCallSignature(function interface{}, args []interface{}) {
	typ := reflect.TypeOf(function)
	if typ.Kind() != reflect.Func {
		panic("Function is not a method!")
	}
	if typ.NumIn() < 1 {
		panic("Casted method has no receiver!")
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

// Asynchronously request that the given function be invoked with the given args.
func (r *Actor) cast(out chan<- Response, function interface{}, args ...interface{}) {
	r.verifyCallSignature(function, args)
	r.runInThread(out, r.receiver, function, args...)
}

func (r *Actor) runInThread(out chan<- Response, receiver reflect.Value, function interface{}, args ...interface{}) {
	if r.queue == nil {
		panic("Call StartActor before sending it messages!")
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

// Start the internal goroutine that powers this actor. Call this function
// before calling Do on this object.
func (r *Actor) startMessageLoop(receiver interface{}) {
	r.queue = NewMessageQueue(kActorQueueLength)
	r.receiver = reflect.ValueOf(receiver)
	go func() {
		var lastReq *Request
		defer func() {
			if e := recover(); e != nil {
				// Actor panicked
				errPanic := &PanicError{PanicErr: e}
				if r.director != nil {
					r.director.removeActor(r.pid)
				}
				r.receiver.Interface().(ActorImplementor).Terminate(errPanic)
				r.queue.Stop <- true
				if lastReq != nil {
					close(lastReq.ReplyTo)
				}
			}
		}()

		for {
			request := <-r.queue.Out
			lastReq = &request
			r.processOneRequest(request)
		}
	}()
}

// TODO(serialx): Develop a novel way to stop an actor (ref: erlang)
//func (r *Actor) StopActor() {
//	// Pass nil function pointer to stop the message loop
//	r.Q.In <- Request{reflect.ValueOf((func())(nil)), nil, nil}
//}
