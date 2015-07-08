package cinema

import "container/list"

type MessageQueue struct {
	queue *list.List
	limit int
	In    chan *Call
	Out   chan *Call
	Stop  chan bool
}

func NewMessageQueue(limit int) *MessageQueue {
	q := new(MessageQueue)
	q.queue = list.New()
	q.limit = limit
	q.In = make(chan *Call)
	q.Out = make(chan *Call)
	q.Stop = make(chan bool)
	go q.Run()
	return q
}

func (q *MessageQueue) processIn(msg *Call) bool {
	if msg.Function.IsNil() {
		return false
	}
	q.queue.PushBack(msg)
	return true
}

func (q *MessageQueue) doIn() bool {
	select {
	case msg := <-q.In:
		return q.processIn(msg)
	case <-q.Stop:
		return false
	}
}

func (q *MessageQueue) doInOut() bool {
	select {
	case msg := <-q.In:
		return q.processIn(msg)
	case q.Out <- q.queue.Front().Value.(*Call):
		q.queue.Remove(q.queue.Front())
	case <-q.Stop:
		return false
	}
	return true
}

func (q *MessageQueue) doOut() bool {
	select {
	case q.Out <- q.queue.Front().Value.(*Call):
		q.queue.Remove(q.queue.Front())
	case <-q.Stop:
		return false
	}
	return true
}

func (q *MessageQueue) Run() {
	defer func() {
		q.drain()
		close(q.In)
		close(q.Out)
	}()

	for {
		if q.queue.Len() == 0 {
			if !q.doIn() {
				break
			}
		} else if q.queue.Len() < q.limit {
			if !q.doInOut() {
				break
			}
		} else {
			if !q.doOut() {
				break
			}
		}
	}
}

func (q *MessageQueue) drain() {
	for {
		select {
		case r := <-q.In:
			close(r.Done)
			continue
		default:
			return
		}
	}
}
