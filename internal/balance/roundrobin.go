package balance

import (
	"sync/atomic"

	"github.com/anrorg/gefahr/internal/backend"
)

// RoundRobin cycles through configured backends while skipping unhealthy ones.
type RoundRobin struct{ next atomic.Uint64 }

// Next returns the next healthy backend in stable configuration order.
func (r *RoundRobin) Next(backends []*backend.Backend) (*backend.Backend, error) {
	if len(backends) == 0 {
		return nil, ErrNoHealthyBackend
	}
	start := int((r.next.Add(1) - 1) % uint64(len(backends)))
	for offset := range len(backends) {
		candidate := backends[(start+offset)%len(backends)]
		if candidate.Alive() {
			return candidate, nil
		}
	}
	return nil, ErrNoHealthyBackend
}
