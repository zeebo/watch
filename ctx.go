package main

import (
	"context"
	"sync"
	"time"
)

func sleep(ctx context.Context, dur time.Duration) bool {
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-ticker.C:
		return true
	}
}

func wait(ctx context.Context, cond *sync.Cond) bool {
	ctx, cancel := context.WithCancel(ctx)
	result := make(chan bool, 1)

	go func() {
		<-ctx.Done()
		result <- false
		cond.Broadcast()
	}()

	go func() {
		cond.Wait()
		result <- true
		cancel()
	}()

	out := <-result
	<-result
	return out
}
