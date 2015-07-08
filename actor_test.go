package cinema

import (
	"strings"
	"testing"
)

type TestActor struct {
	Actor
	t *testing.T
	x int
	y int

	// For BenchmarkChannel test
	in chan AddXRequest

	// For TestPanic test
	shouldStopNormally bool
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
	if a.shouldStopNormally {
		if errReason != ErrActorStop {
			a.t.Errorf("Expected ErrActorStop but got %v\n", errReason)
		}
	} else {
		panicErr, ok := errReason.(*PanicError)
		if ok {
			if panicErr.PanicErr.(int) != a.y {
				a.t.Errorf("Expected PanicError value to be %v but got %v\n", a.y, panicErr.PanicErr)
			}
			return
		} else {
			a.t.Errorf("Expected PanicError but got %v\n", errReason)
		}
	}
}

func (a *TestActor) AddX(x int) int {
	return a.x + x
}

func BenchmarkChannel(b *testing.B) {
	a := TestActor{Actor{}, nil, 5, 10, make(chan AddXRequest), true}
	go a.ProcessAddX()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.GoAddX(3)
	}
	close(a.in)
}

func BenchmarkActor(b *testing.B) {
	a := TestActor{Actor{}, nil, 5, 10, nil, true}
	a.startMessageLoop(&a)
	defer a.stop()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.call((*TestActor).AddX, 3)
	}
}

func TestAddX(t *testing.T) {
	a := TestActor{Actor{}, t, 2, 3, nil, true}
	a.startMessageLoop(&a)
	defer a.stop()

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

	// Stop the actor and see the behaviour after stop
	a.stop()

	r, err = a.call((*TestActor).AddX, 4)
	if err != ErrActorStop {
		t.Errorf("Expected ErrActorStop error, got %v\n", err)
	}
	if r != nil {
		t.Errorf("Expected null return value, got %v result\n", r)
	}

	// cast should success without any errors
	out := make(chan *Call, 1)
	a.cast(out, (*TestActor).AddX, 4)
}

func TestPanic(t *testing.T) {
	a := TestActor{Actor{}, t, 2, 3, nil, false}
	a.startMessageLoop(&a)
	defer a.stop()

	_, err := a.call((*TestActor).DoPanic)
	if err != ErrActorDied {
		t.Errorf("Expected ErrActorDied error, instead got %v\n", err)
	}
}

func TestGetActor(t *testing.T) {
	a := TestActor{Actor{}, t, 2, 3, nil, true}
	if &a.Actor != a.getActor() {
		t.Errorf("getActor is not same as actor\n")
	}
}

func TestVerifyCallSignature(t *testing.T) {
	a := TestActor{Actor{}, t, 2, 3, nil, true}
	a.startMessageLoop(&a)
	defer a.stop()

	testVerify := func(expectedPanic string, function interface{}, args []interface{}) {
		defer func() {
			if err := recover(); err != nil {
				strErr, ok := err.(string)
				if !ok {
					t.Errorf("Unexpected panic: %v\n", err)
					panic(err)
				} else if !strings.HasPrefix(strErr, expectedPanic) {
					t.Errorf("Unexpected panic: %v\n", err)
					panic(err)
				}
			} else {
				t.Errorf("No panic occurred\n")
			}
		}()
		a.verifyCallSignature(function, args)
	}

	testVerify("Function is nil", nil, nil)
	testVerify("Function is not a method", 1, nil)
	lambda := func() {}
	testVerify("Function is not a method. Function has no receiver", lambda, nil)
	lambda2 := func(a TestActor) {}
	testVerify("Cannot assign receiver", lambda2, nil)
	lambda3 := func(a *TestActor, b int) {}
	testVerify("Not enough arguments given", lambda3, []interface{}{})
	testVerify("Too many args for non-variadic function", lambda3, []interface{}{1, 2, 3})
	lambda4 := func(a *TestActor, bs ...int) {}
	testVerify("Cannot assign arg 0", lambda3, []interface{}{"a"})
	a.verifyCallSignature(lambda4, []interface{}{[]int{1, 2, 3, 4}})
}
