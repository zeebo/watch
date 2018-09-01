package main

import (
	"context"
	"sync"
)

type Buffer struct {
	cond *sync.Cond
	data []byte
	gen  int
}

func NewBuffer() *Buffer {
	var mu sync.Mutex
	return &Buffer{cond: sync.NewCond(&mu)}
}

// Write makes buffer an io.Writer, appending into the data slice and signaling waiters.
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	b.data = append(b.data, p...)
	b.gen++
	b.cond.Broadcast()

	return len(p), nil
}

// Clear resets the buffer, bumps the gen, and broadcasts.
func (b *Buffer) Clear() {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	b.data = b.data[:0]
	b.gen++
	b.cond.Broadcast()
}

// Wait will wait until the internal generation is > the passed in gen and return
// a copy of the data and what generation it had.
func (b *Buffer) Wait(ctx context.Context, gen int) (string, int, bool) {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	for gen >= b.gen {
		if !wait(ctx, b.cond) {
			return "", 0, false
		}
	}

	return string(b.data), b.gen, true
}

// Bump just increases the generation and broadcasts.
func (b *Buffer) Bump() {
	b.cond.L.Lock()
	defer b.cond.L.Unlock()

	b.gen++
	b.cond.Broadcast()
}
