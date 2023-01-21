package main_test

import (
	"context"
	"errors"
	"testing"
	"time"

	wk "github.com/solpipe/kiss-kit/work"
)

func TestClock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(func() {
		cancel()
	})

	doneC := ctx.Done()
	config := &wk.Configuration{
		RpcUrl:  "https://api.devnet.solana.com",
		WsUrl:   "wss://api.devnet.solana.com",
		Headers: nil,
	}
	work, err := wk.Create(ctx, config)
	if err != nil {
		t.Fatal(err)
	}

	clock, err := work.Clock()
	if err != nil {
		t.Fatal(err)
	}

	sub := clock.OnSlot()
	defer sub.Unsubscribe()

	u, err := clock.Slot()
	if err != nil {
		t.Fatal(err)
	}
	if u.Slot == 0 {
		t.Fatal("slot cannot be 0")
	}

	target := u.Slot + 10

out:
	for u.Slot < target {
		select {
		case <-doneC:
			err = errors.New("timed out")
		case err = <-sub.ErrorC:
			break out
		case u = <-sub.StreamC:

		}
	}
	if err != nil {
		t.Fatal(err)
	}
}
