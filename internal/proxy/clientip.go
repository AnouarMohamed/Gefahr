package proxy

import (
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/anrorg/gefahr/internal/config"
)

var defaultClientIPHeaders = []string{"X-Forwarded-For", "X-Real-IP"}

type clientIPPolicy struct {
	trusted []netip.Prefix
	headers []string
}

func newClientIPPolicy(cfg config.ClientIP) (*clientIPPolicy, error) {
	policy := &clientIPPolicy{}
	for _, cidr := range cfg.TrustedProxies {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, err
		}
		policy.trusted = append(policy.trusted, prefix)
	}
	policy.headers = cfg.Headers
	if len(policy.trusted) > 0 && len(policy.headers) == 0 {
		policy.headers = defaultClientIPHeaders
	}
	return policy, nil
}

func (p *clientIPPolicy) Identity(r *http.Request) string {
	peer := remoteIP(r.RemoteAddr)
	if peer == "" {
		return "unknown"
	}
	if p == nil || !p.trustedPeer(peer) {
		return peer
	}
	for _, header := range p.headers {
		switch strings.ToLower(header) {
		case "x-forwarded-for":
			if ip := p.fromXForwardedFor(r.Header.Values("X-Forwarded-For")); ip != "" {
				return ip
			}
		case "x-real-ip":
			if ip := validIP(r.Header.Get("X-Real-IP")); ip != "" {
				return ip
			}
		}
	}
	return peer
}

func (p *clientIPPolicy) fromXForwardedFor(values []string) string {
	var chain []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if ip := validIP(part); ip != "" {
				chain = append(chain, ip)
			}
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if !p.trustedPeer(chain[i]) {
			return chain[i]
		}
	}
	if len(chain) > 0 {
		return chain[0]
	}
	return ""
}

func (p *clientIPPolicy) trustedPeer(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, prefix := range p.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddr string) string {
	host := remoteAddr
	if parsed, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = parsed
	}
	return validIP(host)
}

func validIP(value string) string {
	value = strings.TrimSpace(value)
	if host, port, err := net.SplitHostPort(value); err == nil {
		if _, err := strconv.ParseUint(port, 10, 16); err == nil {
			value = host
		}
	}
	value = strings.Trim(value, "[]")
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return ""
	}
	return addr.String()
}
