package workers

import (
	"sync"

	"github.com/panjf2000/ants/v2"
)

type Pool struct {
	ants *ants.Pool
}

func New(size int) (*Pool, error) {
	if size < 1 {
		size = 1
	}
	p, err := ants.NewPool(size)
	if err != nil {
		return nil, err
	}
	return &Pool{ants: p}, nil
}

func (p *Pool) Submit(task func()) error {
	if p == nil || p.ants == nil {
		return ants.ErrPoolClosed
	}
	return p.ants.Submit(task)
}

func (p *Pool) Release() {
	if p == nil || p.ants == nil {
		return
	}
	p.ants.Release()
}

func (p *Pool) Running() int {
	if p == nil || p.ants == nil {
		return 0
	}
	return p.ants.Running()
}

func (p *Pool) Cap() int {
	if p == nil || p.ants == nil {
		return 0
	}
	return p.ants.Cap()
}

var (
	globalMu   sync.RWMutex
	globalPool *Pool
)

func SetGlobal(p *Pool) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalPool = p
}

func Global() *Pool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalPool
}
