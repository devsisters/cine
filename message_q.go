package cinema

import (
	"container/list"
)

type MessageQueue struct {
	queue *list.List
	limit int
	In    chan Request
	Out   chan Request
}

func NewMessageQueue(limit int) *MessageQueue {
	q := new(MessageQueue)
	q.queue = list.New()
	q.limit = limit
	q.In = make(chan Request)
	q.Out = make(chan Request)
	go q.Run()
	return q
}

func (q *MessageQueue) processIn(msg Request) bool {
	if msg.Function.IsNil() {
		q.drain()
		close(q.In)
		close(q.Out)
		return false
	}
	q.queue.PushBack(msg)
	return true
}

func (q *MessageQueue) doIn() bool {
	return q.processIn(<-q.In)
}

func (q *MessageQueue) doInOut() bool {
	select {
	case msg := <-q.In:
		return q.processIn(msg)
	case q.Out <- q.queue.Front().Value.(Request):
		q.queue.Remove(q.queue.Front())
	}
	return true
}

func (q *MessageQueue) doOut() {
	q.Out <- q.queue.Front().Value.(Request)
	q.queue.Remove(q.queue.Front())
}

func (q *MessageQueue) Run() {
	for {
		if q.queue.Len() == 0 {
			if !q.doIn() {
				return
			}
		} else if q.queue.Len() < q.limit {
			if !q.doInOut() {
				return
			}
		} else {
			q.doOut()
		}
	}
}

func (q *MessageQueue) drain() {
	for {
		select {
		case <-q.In:
			continue
		default:
			return
		}
	}
}
