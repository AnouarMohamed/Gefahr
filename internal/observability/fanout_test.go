package observability

import (
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestFanoutForwardsAllObserverTypes(t *testing.T) {
	first := &recordingObserver{}
	second := &recordingObserver{}
	fanout := Fanout{
		Requests: []RequestObserver{first, second},
		Backends: []BackendObserver{first, second},
	}

	fanout.ObserveRequest("id", "api", "GET", "/items", "one", 200, 2, "miss", time.Second)
	fanout.ObserveRateLimit("api", "limited")
	fanout.ObservePolicyDeny("api", "path_denied")
	fanout.SetBackendHealth("api", "one", true)
	fanout.SetBackendActive("api", "one", 3)
	fanout.ReconcileConfig(config.Config{Routes: []config.Route{{Name: "api"}}})

	for _, observer := range []*recordingObserver{first, second} {
		if observer.requests != 1 || observer.rateLimits != 1 || observer.policyDenials != 1 || observer.health != 1 || observer.active != 1 || observer.reconciles != 1 {
			t.Fatalf("observer was not fully notified: %+v", observer)
		}
	}
}

func TestFanoutIgnoresObserversWithoutOptionalRateLimitAndReconcile(t *testing.T) {
	observer := &requestOnlyObserver{}
	fanout := Fanout{Requests: []RequestObserver{observer}}
	fanout.ObserveRateLimit("api", "allowed")
	fanout.ObservePolicyDeny("api", "path_denied")
	fanout.ReconcileConfig(config.Config{})
	fanout.ObserveRequest("id", "api", "GET", "/", "", 200, 1, "bypass", time.Second)
	if observer.requests != 1 {
		t.Fatalf("request observer was not called: %+v", observer)
	}
}

type recordingObserver struct {
	requests      int
	rateLimits    int
	policyDenials int
	health        int
	active        int
	reconciles    int
}

func (o *recordingObserver) ObserveRequest(string, string, string, string, string, int, int, string, time.Duration) {
	o.requests++
}

func (o *recordingObserver) ObserveRateLimit(string, string) { o.rateLimits++ }

func (o *recordingObserver) ObservePolicyDeny(string, string) { o.policyDenials++ }

func (o *recordingObserver) SetBackendHealth(string, string, bool) { o.health++ }

func (o *recordingObserver) SetBackendActive(string, string, int64) { o.active++ }

func (o *recordingObserver) ReconcileConfig(config.Config) { o.reconciles++ }

type requestOnlyObserver struct {
	requests int
}

func (o *requestOnlyObserver) ObserveRequest(string, string, string, string, string, int, int, string, time.Duration) {
	o.requests++
}
