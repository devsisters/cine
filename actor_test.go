package cinema

import (
	"testing"
)

type TestActor struct {
	Actor
	t *testing.T
	x int
	y int

	// For BenchmarkChannel test
	in chan AddXRequest
}

// For BenchmarkChannel test
type AddXRequest struct {
	x   int
	out chan AddXResponse
}

// For BenchmarkChannel test
type AddXResponse struct {
	x   int
	err interface{}
}

// For BenchmarkChannel test
func (a *TestActor) GoAddX(x int) int {
	out := make(chan AddXResponse)
	a.in <- AddXRequest{x, out}
	return (<-out).x
}

// For BenchmarkChannel test
func (a *TestActor) ProcessAddX() {
	for {
		request, ok := <-a.in
		if !ok {
			break
		}
		request.out <- AddXResponse{a.AddX(request.x), nil}
	}
}

func (a *TestActor) DoPanic() int {
	panic(a.y)
}

func (a *TestActor) Terminate(errReason error) {
	panicErr, ok := errReason.(*PanicError)
	if !ok {
		a.t.Errorf("Expected PanicError but got %v\n", errReason)
	}
	if panicErr.PanicErr.(int) != a.y {
		a.t.Errorf("Expected PanicError value to be %v but got %v\n", a.y, panicErr.PanicErr)
	}
}

func (a *TestActor) AddX(x int) int {
	return a.x + x
}

func BenchmarkChannel(b *testing.B) {
	a := TestActor{Actor{}, nil, 5, 10, make(chan AddXRequest)}
	go a.ProcessAddX()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.GoAddX(3)
	}
	close(a.in)
}

func BenchmarkActor(b *testing.B) {
	a := TestActor{Actor{}, nil, 5, 10, nil}
	a.startMessageLoop(&a)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.call((*TestActor).AddX, 3)
	}
}

func TestAddX(t *testing.T) {
	a := TestActor{Actor{}, t, 2, 3, nil}
	a.startMessageLoop(&a)

	r, err := a.call((*TestActor).AddX, 4)
	if err != nil {
		t.Errorf("Expected no error, got %v\n", err)
	}
	if len(r) != 1 {
		t.Errorf("Expected 1 return values, got %d results\n", len(r))
	}
	if x := r[0].(int); x != 6 {
		t.Errorf("Expected x = %v, actual %v\n", 6, x)
	}
}

func TestPanic(t *testing.T) {
	a := TestActor{Actor{}, t, 2, 3, nil}
	a.startMessageLoop(&a)

	_, err := a.call((*TestActor).DoPanic)
	if err != ErrActorDied {
		t.Errorf("Expected ErrActorDied error, instead got %v\n", err)
	}
}
