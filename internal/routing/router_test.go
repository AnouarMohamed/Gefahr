package routing

import (
	"testing"

	"github.com/anrorg/gefahr/internal/config"
)

func TestCandidatesMatchesHostCaseInsensitivelyWithoutPort(t *testing.T) {
	router := New([]config.Route{{Name: "api", Host: "API.Example.Test"}, {Name: "other", Host: "other.test"}})
	got := router.Candidates("api.example.test:8080")
	if len(got) != 1 || got[0].Name != "api" {
		t.Fatalf("candidates = %#v", got)
	}
}

func TestCandidatesIncludesCatchAll(t *testing.T) {
	router := New([]config.Route{{Name: "default"}, {Name: "api", Host: "api.test"}})
	got := router.Candidates("unknown.test")
	if len(got) != 1 || got[0].Name != "default" {
		t.Fatalf("candidates = %#v", got)
	}
}

func TestMatchUsesLongestBoundarySafePrefix(t *testing.T) {
	router := New([]config.Route{
		{Name: "root", PathPrefix: "/"},
		{Name: "api", PathPrefix: "/api"},
		{Name: "v1", PathPrefix: "/api/v1"},
	})
	for _, tc := range []struct{ path, want string }{
		{"/api/v1/users", "v1"},
		{"/api/users", "api"},
		{"/apix", "root"},
	} {
		got, ok := router.Match("example.test", tc.path)
		if !ok || got.Name != tc.want {
			t.Fatalf("Match(%q) = %#v, %v; want %s", tc.path, got, ok, tc.want)
		}
	}
}

func TestMatchPrefersExactHostOnTie(t *testing.T) {
	router := New([]config.Route{{Name: "default", PathPrefix: "/"}, {Name: "api", Host: "api.test", PathPrefix: "/"}})
	got, ok := router.Match("api.test", "/")
	if !ok || got.Name != "api" {
		t.Fatalf("match = %#v, %v", got, ok)
	}
}
