package main

import (
	"context"
)

type dataBroker struct {
	events  chan []byte
	clients map[chan []byte]struct{}
	add     chan chan []byte
	remove  chan chan []byte
}

func newBroker() *dataBroker {
	return &dataBroker{
		events:  make(chan []byte, 1),
		clients: make(map[chan []byte]struct{}),
		remove:  make(chan chan []byte),
		add:     make(chan chan []byte),
	}
}
func (b *dataBroker) Remove(c chan []byte) {
	b.remove <- c
}

func (b *dataBroker) Add(c chan []byte) {
	b.add <- c
}

func (b *dataBroker) Publish(evt []byte) {
	b.events <- evt
}

func (b *dataBroker) Run(ctx context.Context) error {
	for {
		select {
		case c := <-b.remove:
			delete(b.clients, c)
		case c := <-b.add:
			b.clients[c] = struct{}{}
		case <-ctx.Done():
			return nil
		case evt := <-b.events:
			for c := range b.clients {
				c <- evt
			}
		}
	}
}
