package proxy

import (
	"net/http"
	"strings"

	"github.com/anrorg/gefahr/internal/config"
)

const (
	policyReasonMethodNotAllowed      = "method_not_allowed"
	policyReasonPathDenied            = "path_denied"
	policyReasonRequiredHeaderMissing = "required_header_missing"
	policyReasonHeaderDenied          = "header_denied"
	policyReasonQueryTooLarge         = "query_too_large"
)

type policyDenial struct {
	status       int
	code         string
	message      string
	allowMethods []string
}

func evaluateRoutePolicy(policy config.RoutePolicy, r *http.Request) (policyDenial, bool) {
	if policy.MaxQueryBytes > 0 && len(r.URL.RawQuery) > policy.MaxQueryBytes {
		return policyDenial{status: http.StatusRequestURITooLong, code: policyReasonQueryTooLarge, message: "request query string too large"}, true
	}
	if len(policy.AllowedMethods) > 0 && !methodAllowed(policy.AllowedMethods, r.Method) {
		return policyDenial{status: http.StatusMethodNotAllowed, code: policyReasonMethodNotAllowed, message: "request method is not allowed", allowMethods: policy.AllowedMethods}, true
	}
	for _, prefix := range policy.DeniedPathPrefixes {
		if policyPathMatches(prefix, r.URL.Path) {
			return policyDenial{status: http.StatusForbidden, code: policyReasonPathDenied, message: "request path is denied"}, true
		}
	}
	for _, header := range policy.RequiredHeaders {
		if strings.TrimSpace(r.Header.Get(header)) == "" {
			return policyDenial{status: http.StatusBadRequest, code: policyReasonRequiredHeaderMissing, message: "required request header is missing"}, true
		}
	}
	for _, header := range policy.DeniedHeaders {
		if len(r.Header.Values(header)) > 0 {
			return policyDenial{status: http.StatusForbidden, code: policyReasonHeaderDenied, message: "request header is denied"}, true
		}
	}
	return policyDenial{}, false
}

func methodAllowed(allowed []string, method string) bool {
	for _, candidate := range allowed {
		if candidate == method {
			return true
		}
	}
	return false
}

func policyPathMatches(prefix, path string) bool {
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/")
}
