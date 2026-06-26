package balance

import (
	"sync/atomic"

	"github.com/anrorg/gefahr/internal/backend"
)

// LeastConnections selects the healthy backend with the fewest active
// requests. Equal candidates rotate to avoid a permanent first-item bias.
type LeastConnections struct{ tie atomic.Uint64 }

// Next returns a least-loaded healthy backend.
func (l *LeastConnections) Next(backends []*backend.Backend) (*backend.Backend, error) {
	var candidates []*backend.Backend
	var minimum int64
	for _, candidate := range backends {
		if !candidate.Alive() {
			continue
		}
		active := candidate.Active()
		if len(candidates) == 0 || active < minimum {
			minimum = active
			candidates = candidates[:0]
			candidates = append(candidates, candidate)
		} else if active == minimum {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return nil, ErrNoHealthyBackend
	}
	index := (l.tie.Add(1) - 1) % uint64(len(candidates))
	return candidates[index], nil
}
