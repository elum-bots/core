package apiflow

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Class string

const (
	ClassResponse     Class = "response"
	ClassBroadcast    Class = "broadcast"
	ClassSubscription Class = "subscription"
	ClassAdmin        Class = "admin"
)

var (
	ErrInvalidRPS     = errors.New("invalid rps")
	ErrQueueOverflow  = errors.New("api queue overflow")
	ErrDispatcherDone = errors.New("dispatcher closed")
)

type Options struct {
	RPS               int
	QueueSize         int
	ResponseShare     int
	BroadcastShare    int
	SubscriptionShare int
	AdminShare        int
	AdminReserveRPS   int
}

type request struct {
	ctx   context.Context
	ready chan struct{}
}

type Dispatcher struct {
	rps       int
	queues    map[Class]chan *request
	schedule  []Class
	nextIndex int

	adminReserveRPS int
	windowStart     time.Time
	adminGranted    int

	done chan struct{}
	once sync.Once
}

type Stats struct {
	QueueResponse     int
	QueueBroadcast    int
	QueueSubscription int
	QueueAdmin        int
	QueueTotal        int
	QueueCapacity     int
}

func New(opts Options) (*Dispatcher, error) {
	if opts.RPS <= 0 {
		return nil, ErrInvalidRPS
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 2048
	}
	weights := normalizeWeights(opts.ResponseShare, opts.BroadcastShare, opts.SubscriptionShare, opts.AdminShare)
	schedule := makeSchedule(weights[ClassResponse], weights[ClassBroadcast], weights[ClassSubscription], weights[ClassAdmin])
	if opts.AdminReserveRPS < 0 {
		opts.AdminReserveRPS = 0
	}

	d := &Dispatcher{
		rps: opts.RPS,
		queues: map[Class]chan *request{
			ClassResponse:     make(chan *request, opts.QueueSize),
			ClassBroadcast:    make(chan *request, opts.QueueSize),
			ClassSubscription: make(chan *request, opts.QueueSize),
			ClassAdmin:        make(chan *request, opts.QueueSize),
		},
		schedule:        schedule,
		adminReserveRPS: opts.AdminReserveRPS,
		windowStart:     time.Now(),
		done:            make(chan struct{}),
	}
	go d.loop()
	return d, nil
}

func normalizeWeights(response, broadcast, subscription, admin int) map[Class]int {
	if response < 0 {
		response = 0
	}
	if broadcast < 0 {
		broadcast = 0
	}
	if subscription < 0 {
		subscription = 0
	}
	if admin < 0 {
		admin = 0
	}
	if response+broadcast+subscription+admin == 0 {
		response, broadcast, subscription = 40, 10, 50
	}
	if admin == 0 {
		admin = 1
	}
	return map[Class]int{
		ClassResponse:     response,
		ClassBroadcast:    broadcast,
		ClassSubscription: subscription,
		ClassAdmin:        admin,
	}
}

func makeSchedule(response, broadcast, subscription, admin int) []Class {
	out := make([]Class, 0, response+broadcast+subscription+admin)
	for i := 0; i < response; i++ {
		out = append(out, ClassResponse)
	}
	for i := 0; i < broadcast; i++ {
		out = append(out, ClassBroadcast)
	}
	for i := 0; i < subscription; i++ {
		out = append(out, ClassSubscription)
	}
	for i := 0; i < admin; i++ {
		out = append(out, ClassAdmin)
	}
	if len(out) == 0 {
		return []Class{ClassResponse}
	}
	return out
}

func (d *Dispatcher) Close() {
	d.once.Do(func() {
		close(d.done)
	})
}

func (d *Dispatcher) Do(ctx context.Context, class Class, fn func(context.Context) error) error {
	if err := d.Acquire(ctx, class); err != nil {
		return err
	}
	return fn(ctx)
}

func (d *Dispatcher) Acquire(ctx context.Context, class Class) error {
	q, ok := d.queues[class]
	if !ok {
		q = d.queues[ClassResponse]
	}
	req := &request{ctx: ctx, ready: make(chan struct{})}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.done:
		return ErrDispatcherDone
	default:
	}

	select {
	case q <- req:
	default:
		return ErrQueueOverflow
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.done:
		return ErrDispatcherDone
	case <-req.ready:
		return nil
	}
}

func (d *Dispatcher) loop() {
	interval := time.Second / time.Duration(d.rps)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-d.done:
			return
		case <-t.C:
			d.grantOne()
		}
	}
}

func (d *Dispatcher) grantOne() {
	now := time.Now()
	d.maybeRotateWindow(now)
	if d.adminReserveRPS > 0 && d.adminGranted < d.adminReserveRPS {
		if d.grantFromClass(ClassAdmin) {
			d.adminGranted++
			return
		}
	}

	n := len(d.schedule)
	if n == 0 {
		return
	}
	for i := 0; i < n; i++ {
		cls := d.schedule[d.nextIndex]
		d.nextIndex = (d.nextIndex + 1) % n
		if d.grantFromClass(cls) {
			if cls == ClassAdmin {
				d.adminGranted++
			}
			return
		}
	}
}

func (d *Dispatcher) grantFromClass(class Class) bool {
	q, ok := d.queues[class]
	if !ok {
		return false
	}
	for {
		select {
		case req := <-q:
			select {
			case <-req.ctx.Done():
				continue
			default:
				close(req.ready)
				return true
			}
		default:
			return false
		}
	}
}

func (d *Dispatcher) maybeRotateWindow(now time.Time) {
	if now.Sub(d.windowStart) < time.Second {
		return
	}
	d.windowStart = now
	d.adminGranted = 0
}

func (d *Dispatcher) Stats() Stats {
	resp := len(d.queues[ClassResponse])
	bcast := len(d.queues[ClassBroadcast])
	subs := len(d.queues[ClassSubscription])
	admin := len(d.queues[ClassAdmin])
	capacity := cap(d.queues[ClassResponse])
	return Stats{
		QueueResponse:     resp,
		QueueBroadcast:    bcast,
		QueueSubscription: subs,
		QueueAdmin:        admin,
		QueueTotal:        resp + bcast + subs + admin,
		QueueCapacity:     capacity,
	}
}
