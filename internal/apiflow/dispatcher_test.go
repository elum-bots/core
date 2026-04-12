package apiflow

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcherEnforcesRPS(t *testing.T) {
	d, err := New(Options{
		RPS:               20,
		QueueSize:         4096,
		ResponseShare:     40,
		BroadcastShare:    10,
		SubscriptionShare: 50,
	})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	defer d.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1300*time.Millisecond)
	defer cancel()

	var okCount atomic.Int64
	var wg sync.WaitGroup
	for range 300 {
		wg.Go(func() {
			if err := d.Acquire(ctx, ClassResponse); err == nil {
				okCount.Add(1)
			}
		})
	}
	wg.Wait()

	got := okCount.Load()
	if got > 30 {
		t.Fatalf("too many permits granted for configured RPS: got=%d", got)
	}
	if got < 15 {
		t.Fatalf("too few permits granted, dispatcher appears stalled: got=%d", got)
	}
}

func TestDispatcherWeightedFairness(t *testing.T) {
	d, err := New(Options{
		RPS:               120,
		QueueSize:         4096,
		ResponseShare:     40,
		BroadcastShare:    10,
		SubscriptionShare: 50,
		AdminShare:        1,
		AdminReserveRPS:   1,
	})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	defer d.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var responseCount atomic.Int64
	var broadcastCount atomic.Int64
	var subscriptionCount atomic.Int64

	var wg sync.WaitGroup
	pump := func(class Class, counter *atomic.Int64) {
		defer wg.Done()
		for {
			if err := d.Acquire(ctx, class); err != nil {
				return
			}
			counter.Add(1)
		}
	}

	wg.Add(3)
	go pump(ClassResponse, &responseCount)
	go pump(ClassBroadcast, &broadcastCount)
	go pump(ClassSubscription, &subscriptionCount)
	wg.Wait()

	resp := float64(responseCount.Load())
	bcast := float64(broadcastCount.Load())
	subs := float64(subscriptionCount.Load())
	total := resp + bcast + subs

	if total < 80 {
		t.Fatalf("too few total permits for fairness test: total=%.0f", total)
	}

	respShare := resp / total
	bcastShare := bcast / total
	subsShare := subs / total

	if respShare < 0.30 || respShare > 0.50 {
		t.Fatalf("response share out of range: got=%.3f", respShare)
	}
	if bcastShare < 0.05 || bcastShare > 0.18 {
		t.Fatalf("broadcast share out of range: got=%.3f", bcastShare)
	}
	if subsShare < 0.40 || subsShare > 0.60 {
		t.Fatalf("subscription share out of range: got=%.3f", subsShare)
	}
}

func TestDispatcherAdminReserveAlwaysGetsSlot(t *testing.T) {
	d, err := New(Options{
		RPS:               25,
		QueueSize:         4096,
		ResponseShare:     40,
		BroadcastShare:    10,
		SubscriptionShare: 50,
		AdminShare:        1,
		AdminReserveRPS:   1,
	})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	defer d.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2300*time.Millisecond)
	defer cancel()

	var adminCount atomic.Int64
	var userCount atomic.Int64
	var wg sync.WaitGroup

	// Flood non-admin queues.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if err := d.Acquire(ctx, ClassResponse); err != nil {
					return
				}
				userCount.Add(1)
			}
		}()
	}

	// Keep admin queue busy too.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := d.Acquire(ctx, ClassAdmin); err != nil {
				return
			}
			adminCount.Add(1)
		}
	}()

	wg.Wait()

	if userCount.Load() < 10 {
		t.Fatalf("test setup failed: non-admin queue did not get enough load, got=%d", userCount.Load())
	}
	if adminCount.Load() < 2 {
		t.Fatalf("admin reserve not respected, expected at least 2 admin grants in ~2s, got=%d", adminCount.Load())
	}
}
