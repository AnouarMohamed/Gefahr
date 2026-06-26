package balance

import (
	"net/url"
	"testing"

	"github.com/anrorg/gefahr/internal/backend"
)

func testBackends(names ...string) []*backend.Backend {
	result := make([]*backend.Backend, 0, len(names))
	for _, name := range names {
		result = append(result, backend.New(name, &url.URL{Scheme: "http", Host: name}))
	}
	return result
}

func TestRoundRobinCyclesAndSkipsDeadBackends(t *testing.T) {
	backends := testBackends("a", "b", "c")
	backends[1].SetAlive(false)
	rr := &RoundRobin{}
	for i, want := range []string{"a", "c", "c", "a"} {
		got, err := rr.Next(backends)
		if err != nil || got.Name() != want {
			t.Fatalf("selection %d = %v, %v; want %s", i, got, err, want)
		}
	}
}

func TestRoundRobinReportsNoHealthyBackend(t *testing.T) {
	backends := testBackends("a")
	backends[0].SetAlive(false)
	if _, err := new(RoundRobin).Next(backends); err != ErrNoHealthyBackend {
		t.Fatalf("error = %v", err)
	}
}
