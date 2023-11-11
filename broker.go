package chat

import (
	"fmt"
	"sync"
)

type printer interface {
	Printf(fmt string, args ...interface{})
}

type Broker struct {
	events map[string][]chan string
	mtx    *sync.RWMutex
	logger printer
}

func NewBroker(logger printer) *Broker {
	return &Broker{
		events: make(map[string][]chan string),
		mtx:    &sync.RWMutex{},
		logger: logger,
	}
}
func (b *Broker) RemoveListener(channel string, c chan string) {
	b.mtx.Lock()
	removed := false
	ch, ok := b.events[channel]
	if ok {
		for i := 0; i < len(ch); i++ {
			if ch[i] == c {
				ch = append(ch[:i], ch[i+1:]...)
				removed = true
				break
			}
		}
		b.events[channel] = ch
	}
	b.mtx.Unlock()
	if removed {
		fmt.Printf("removed listener from: %s\n", channel)
	}
}

func (b *Broker) AddListener(channel string, c chan string) func() {
	b.mtx.Lock()
	add := true
	ch, ok := b.events[channel]
	if ok {
		for i := 0; i < len(ch); i++ {
			if ch[i] == c {
				add = false
				break
			}
		}
	}
	if add {
		ch = append(ch, c)
		b.events[channel] = ch
	}
	b.mtx.Unlock()
	if !ok {
		fmt.Printf("New channel: %s\n", channel)
	}
	fmt.Printf("added listener for: %s\n", channel)

	return func() {
		b.RemoveListener(channel, c)
	}
}

func (b *Broker) PublishEvent(channel, event string) {
	b.mtx.RLock()
	if ch, ok := b.events[channel]; ok {
		b.mtx.RUnlock()
		for _, v := range ch {
			select {
			case v <- event:
			default:
				fmt.Printf("event dropped: %s\n", channel)
			}
		}
		return
	}
	b.mtx.RUnlock()
}
