// Package broadcast provides a small pub/sub fan-out used to serve gRPC
// streaming RPCs: one background poll loop publishes fetched values, and any
// number of concurrent stream subscribers receive them without each
// triggering their own upstream fetch.
package broadcast

import "sync"

// Broadcaster fans out published values of type T to any number of
// subscribers, caching the latest value so a new subscriber receives it
// immediately rather than waiting for the next publish.
type Broadcaster[T any] struct {
	mu      sync.Mutex
	subs    map[chan T]struct{}
	latest  T
	hasLast bool
}

func New[T any]() *Broadcaster[T] {
	return &Broadcaster[T]{subs: make(map[chan T]struct{})}
}

// Subscribe registers a new subscriber, immediately delivering the latest
// published value (if any) into the returned channel.
func (b *Broadcaster[T]) Subscribe() chan T {
	ch := make(chan T, 1)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	if b.hasLast {
		ch <- b.latest
	}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel returned by Subscribe.
func (b *Broadcaster[T]) Unsubscribe(ch chan T) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
	close(ch)
}

// Publish sends v to every current subscriber and caches it for future
// subscribers. Never blocks: a subscriber that hasn't drained its previous
// value has it replaced rather than stalling the publisher.
func (b *Broadcaster[T]) Publish(v T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.latest = v
	b.hasLast = true
	for ch := range b.subs {
		select {
		case ch <- v:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- v:
			default:
			}
		}
	}
}
