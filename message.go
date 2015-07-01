package cinema;

import(
	"fmt"
	"reflect"
	"runtime"
)

// Represents a request to an actor's thread to invoke the given function with
// the given arguments.
type Request struct {
	Function reflect.Value
	Args     []reflect.Value
	ReplyTo chan<- Response
}

type Response interface {
	Panicked() bool
	PanicCause() interface{}
	Interpret() []reflect.Value
	InterpretAsInterfaces() []interface{}
}

// Represents the result of a function invocation.
type ResponseImpl struct {
	result   []reflect.Value // The return value of the function.
	err      interface{}     // The value passed to panic, if it was called.
	panicked bool            // True if the invocation called panic.
	Stack    []byte
	function reflect.Value
	args     []reflect.Value
}

func (r ResponseImpl) Panicked() bool {
	return r.panicked
}

func (r ResponseImpl) PanicCause() interface{} {
	if !r.panicked {
		panic("Panic Cause not available")
	}

	return r.err
}

func (r ResponseImpl) PanicStack() {
	fmt.Printf("Panic occurred while calling %s(", runtime.FuncForPC(r.function.Pointer()).Name())
	for i, x := range r.args {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("%s", x.Type().Name())
	}
	fmt.Printf("):\n%s\n", r.Stack)
}

// If the response indicates that the executor panicked, replicate the panic
// on this thread. Otherwise, return the result.
func (r ResponseImpl) Interpret() []reflect.Value {
	if r.panicked {
		panic(r)
	}
	return r.result
}

func (r ResponseImpl) InterpretAsInterfaces() []interface{} {
	values := r.Interpret()
	interfaces := make([]interface{}, len(values))
	for i, x := range values {
		interfaces[i] = x.Interface()
	}
	return interfaces
}
