package handler_test

import (
	"context"
	"strings"
	"testing"

	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// /metrics hanya boleh ada di listener ops internal. Di router publik path itu cuma dianggap alias
// biasa (catch-all /:alias), jadi tidak pernah mengembalikan metrik Prometheus.
func TestPublicRouter_DoesNotExposeMetrics(t *testing.T) {
	env := newEnv()
	env.link.resolveAlias = func(context.Context, *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
		return nil, status.Error(codes.NotFound, "link not found")
	}
	w := env.do(t, "GET", "/metrics", "")
	assertError(t, w, 404, "LINK_NOT_FOUND")
	if strings.Contains(w.Body.String(), "go_goroutines") || strings.Contains(w.Body.String(), "# HELP") {
		t.Error("public router leaked prometheus metrics")
	}
}

func TestResponses_CarryRequestID(t *testing.T) {
	env := newEnv()
	w := env.do(t, "GET", "/healthz", "")
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("response must carry X-Request-ID")
	}
}
