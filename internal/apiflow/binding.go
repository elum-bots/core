package apiflow

import (
	"context"
	"sync"
)

type ctxClassKey struct{}

func WithClass(ctx context.Context, class Class) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxClassKey{}, class)
}

func ClassFromContext(ctx context.Context) Class {
	if ctx == nil {
		return ClassResponse
	}
	v, _ := ctx.Value(ctxClassKey{}).(Class)
	switch v {
	case ClassBroadcast, ClassSubscription, ClassResponse, ClassAdmin:
		return v
	default:
		return ClassResponse
	}
}

var (
	globalMu            sync.RWMutex
	globalDispatcher    *Dispatcher
	globalMAXDispatcher *Dispatcher
	globalTGDispatcher  *Dispatcher
)

func SetGlobal(d *Dispatcher) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalDispatcher = d
}

func SetMAX(d *Dispatcher) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalMAXDispatcher = d
}

func SetTG(d *Dispatcher) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalTGDispatcher = d
}

func Global() *Dispatcher {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalDispatcher
}

func MAX() *Dispatcher {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalMAXDispatcher != nil {
		return globalMAXDispatcher
	}
	return globalDispatcher
}

func TG() *Dispatcher {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalTGDispatcher != nil {
		return globalTGDispatcher
	}
	return globalDispatcher
}
