package cinema;

import(
	"reflect"
)

type RetryBehavior int32
const(
	kRetryNever = iota
	kRetryAlways
)

type Cache struct {
	cache map[string]interface{}
	pending map[string]*PendingCacheEntry
	retryBehavior RetryBehavior
	actor *Actor
}

func NewCache(actor *Actor, retryBehavior RetryBehavior) Cache {
	return Cache{retryBehavior: retryBehavior, actor: actor}
}

func (c *Cache) IsLoaded(key string) bool {
	_, ok := c.cache[key]
	return ok
}

func (c *Cache) IsPending(key string) bool {
	_, ok := c.pending[key]
	return ok
}

// Retrieves the given key from the cache.
func (c *Cache) Get(key string) (interface{}, Future) {
	if value, ok := c.cache[key]; ok {
		return value, nil
	} else if pending, isPending := c.pending[key]; isPending {
		return nil, pending.addWatcher()
	}
	return nil, nil
}

func (c *Cache) Load(key string) Promise {
	if _, ok := c.pending[key]; ok {
		panic("Do not load an already-loaded value")
	}

	pending := newPendingCacheEntry(c)
	c.pending[key] = pending
	return pending
}

type Promise interface {
	Fulfill(x interface{})
}

type PendingCacheEntry struct {
	key string
	watchers []Watcher
  cache *Cache
}

func newPendingCacheEntry(c *Cache) *PendingCacheEntry {
	return &PendingCacheEntry{cache: c, watchers: make([]Watcher,0,5)}
}

func (p *PendingCacheEntry) addWatcher() Future {
	watcher := newWatcher()
	p.watchers = append(p.watchers, watcher)
	return &p.watchers[len(p.watchers) - 1]
}

func (e *PendingCacheEntry) Fulfill(x interface{}) {
	e.cache.actor.runInThread(
		nil, reflect.ValueOf(e), (*PendingCacheEntry).notifyListeners, x)
}

func (e *PendingCacheEntry) notifyListeners(x interface{}) {
	delete(e.cache.pending, e.key)
	e.cache.cache[e.key] = x
	for _, w := range e.watchers {
		w.out <- x
	}
}

type Watcher struct {
	out chan interface{}
	worker interface{}
	args []interface{}
}

type Future interface {
	Get() interface{}
}

func newWatcher() Watcher {
	return Watcher{out: make(chan interface{}, 1)}
}

func (w *Watcher) Get() interface{} {
	x := <- w.out
	w.out <- x
	return x
}
