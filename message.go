package cine

import "reflect"

// Represents a request to an actor's thread to invoke the given function with
// the given arguments.
type ActorCall struct {
	Function reflect.Value
	Args     []reflect.Value
	Reply    []reflect.Value
	Done     chan *ActorCall
}

func (c ActorCall) ReplyAsInterfaces() []interface{} {
	values := c.Reply
	interfaces := make([]interface{}, len(values))
	for i, x := range values {
		interfaces[i] = x.Interface()
	}
	return interfaces
}
