// Package balance implements pluggable healthy-backend selection policies.
package balance

import (
	"errors"

	"github.com/anrorg/gefahr/internal/backend"
)

// ErrNoHealthyBackend means no configured backend can receive traffic.
var ErrNoHealthyBackend = errors.New("no healthy backend")

// Balancer selects one backend without changing its active request count.
type Balancer interface {
	Next([]*backend.Backend) (*backend.Backend, error)
}
