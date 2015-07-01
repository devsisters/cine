package glam

import (
	"testing"
)

type A struct {
	x  int
	y  int
	in chan GetXRequest
	Actor
}

type GetXRequest struct {
	x   int
	out chan GetXResponse
}

type GetXResponse struct {
	x   int
	err interface{}
}

func (a A) GoX(x int) int {
	out := make(chan GetXResponse)
	a.in <- GetXRequest{x, out}
	return (<-out).x
}

func (a A) ProcessGetX() {
	for {
		request := <-a.in
		request.out <- GetXResponse{a.GetX(request.x), nil}
	}
}

func (a A) GetX(x int) int {
	return a.x + x
}

func (a A) DoPanic() int {
	panic(a.y)
}

func TestGetX(t *testing.T) {
	a := A{2, 3, nil, Actor{}}
	a.startMessageLoop(a)

	if x := a.Call(A.GetX, 4)[0].(int); x != 6 {
		t.Errorf("Expected x = %v, actual %v\n", 6, x)
	}
}

func TestPanic(t *testing.T) {
	a := A{2, 3, nil, Actor{}}
	a.startMessageLoop(a)

	defer func() {
		if e := recover(); e != nil {
			if response, ok := e.(Response); ok {
				if panicCause, ok2 := response.PanicCause().(int); !ok2 || panicCause != 3 {
					t.Errorf("Expected to receive panic response == 3, actual %v\n", panicCause)
				}
			} else {
				t.Errorf("Expected panic(Response), actual %v\n", e)
			}
		}
	}()

	a.Call(A.DoPanic)
}

func BenchmarkActor(b *testing.B) {
	b.StopTimer()
	a := A{5, 10, nil, Actor{}}
	a.startMessageLoop(a)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		a.Call(A.GetX, 3)
	}
}

func BenchmarkChannel(b *testing.B) {
	b.StopTimer()
	a := A{5, 10, make(chan GetXRequest), Actor{}}
	go a.ProcessGetX()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		a.GoX(3)
	}
}
