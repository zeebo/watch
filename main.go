package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/zeebo/errs"
	"github.com/zeebo/term"
)

// Error wraps all of the returned errors for a nice message and traceback.
var Error = errs.Class("watch")

// flags is a global struct of the flags in the process.
var flags struct {
	n int
}

func init() {
	flag.IntVar(&flags.n, "n", 1, "Seconds to wait between updates")
}

func main() {
	flag.Parse()

	// create the parent context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up signals to cancel the context
	sig := make(chan os.Signal, 10)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() { <-sig; cancel() }()

	// run our command
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(1)
	}
}

// run is the main loop of the program.
func run(ctx context.Context) (err error) {
	if err := term.Init(); err != nil {
		return Error.Wrap(err)
	}
	defer term.Close()

	b := newBuffer()

	// launch our goroutines to handle the screen
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { generateFrames(ctx, b); wg.Done() }()
	go func() { blitFrames(ctx, b); wg.Done() }()
	go func() { watchRedraw(ctx, b); wg.Done() }()

	wg.Wait()
	return nil
}

// generateFrames runs the commands and appends the data into the buffer.
func generateFrames(ctx context.Context, b *buffer) {
	args := flag.Args()

	for {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = b
		cmd.Stderr = b

		b.Clear()
		fmt.Fprintf(b, "%v | every %d seconds | %s\n\n", time.Now(), flags.n, strings.Join(args, " "))

		if err := cmd.Run(); err != nil {
			fmt.Fprintln(b, "\n", err)
		}

		if !sleep(ctx, time.Duration(flags.n)*time.Second) {
			return
		}
	}
}

// blitFrames watches the buffer and displays any changes to the screen.
func blitFrames(ctx context.Context, b *buffer) {
	var (
		data string
		gen  int
		ok   bool
	)

	for {
		data, gen, ok = b.Wait(ctx, gen)
		if !ok {
			return
		}

		term.Clear()
		x, y := 0, 0

		for _, r := range data {
			term.Set(x, y, r)

			switch r {
			case '\n':
				x, y = 0, y+1
			case '\r':
				x = 0
			default:
				w := runewidth.RuneWidth(r)
				if w == 0 || (w == 2 && runewidth.IsAmbiguousWidth(r)) {
					w = 1
				}
				x += w
			}
		}

		term.Flush()
	}
}

// watchRedraw is responsible for signaling when the display may need to be redrawn.
func watchRedraw(ctx context.Context, b *buffer) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)

	for {
		select {
		case <-sig:
			b.Bump()
		case <-ctx.Done():
			return
		}
	}
}
