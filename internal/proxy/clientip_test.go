package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anrorg/gefahr/internal/config"
)

func TestClientIPPolicyAcceptsForwardedForWithPorts(t *testing.T) {
	policy := clientIPPolicyForTest(t)
	request := httptest.NewRequest(http.MethodGet, "http://api.test/", nil)
	request.RemoteAddr = "10.0.0.5:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.10:5678, 10.0.0.5:443")
	if got := policy.Identity(request); got != "203.0.113.10" {
		t.Fatalf("identity = %q", got)
	}
}

func TestClientIPPolicyAcceptsBracketedIPv6ForwardedForWithPort(t *testing.T) {
	policy := clientIPPolicyForTest(t)
	request := httptest.NewRequest(http.MethodGet, "http://api.test/", nil)
	request.RemoteAddr = "10.0.0.5:4321"
	request.Header.Set("X-Forwarded-For", "[2001:db8::7]:5678, 10.0.0.5")
	if got := policy.Identity(request); got != "2001:db8::7" {
		t.Fatalf("identity = %q", got)
	}
}

func TestClientIPPolicyAcceptsRealIPWithPort(t *testing.T) {
	policy := clientIPPolicyForTest(t)
	request := httptest.NewRequest(http.MethodGet, "http://api.test/", nil)
	request.RemoteAddr = "10.0.0.5:4321"
	request.Header.Set("X-Real-IP", "198.51.100.3:1234")
	if got := policy.Identity(request); got != "198.51.100.3" {
		t.Fatalf("identity = %q", got)
	}
}

func clientIPPolicyForTest(t *testing.T) *clientIPPolicy {
	t.Helper()
	policy, err := newClientIPPolicy(config.ClientIP{TrustedProxies: []string{"10.0.0.0/8"}})
	if err != nil {
		t.Fatal(err)
	}
	return policy
}
