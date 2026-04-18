// Package svc provides the API gateway service context.
package svc

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"

	"sovereign-ai-compliance/api-gateway/internal/config"
	"sovereign-ai-compliance/api-gateway/internal/middleware"

	"github.com/zeromicro/go-zero/core/breaker"
	"github.com/zeromicro/go-zero/core/logx"
)

// ServiceContext holds the gateway runtime dependencies.
type ServiceContext struct {
	Config          config.Config
	JWTAuth         func(http.HandlerFunc) http.HandlerFunc
	Tenant          func(http.HandlerFunc) http.HandlerFunc
	UpstreamProxies map[string]*httputil.ReverseProxy
	Breakers        map[string]breaker.Breaker
}

// NewServiceContext creates a ServiceContext from configuration.
func NewServiceContext(c config.Config) *ServiceContext {
	proxies := make(map[string]*httputil.ReverseProxy)
	breakers := make(map[string]breaker.Breaker)
	upstreams := map[string]string{
		"auth":         c.Upstream.Auth,
		"org":          c.Upstream.Org,
		"repo":         c.Upstream.Repo,
		"rag":          c.Upstream.RAG,
		"doc":          c.Upstream.Doc,
		"audit":        c.Upstream.Audit,
		"agent":        c.Upstream.Agent,
		"notification": c.Upstream.Notification,
	}

	for name, target := range upstreams {
		if target == "" {
			continue
		}
		u, err := url.Parse(target)
		if err != nil {
			logx.Errorf("failed to parse upstream URL for %s: %v", name, err)
			continue
		}
		proxies[name] = httputil.NewSingleHostReverseProxy(u)
		breakers[name] = breaker.NewBreaker(breaker.WithName("upstream-" + name))
	}

	return &ServiceContext{
		Config:          c,
		JWTAuth:         middleware.JWTAuth(c.Auth.PublicKeyPath, c.PublicEndpoints),
		Tenant:          middleware.Tenant,
		UpstreamProxies: proxies,
		Breakers:        breakers,
	}
}

// Proxy returns a handler that forwards requests to the named upstream service.
// It wraps the call with a circuit breaker to prevent cascading failures.
func (sc *ServiceContext) Proxy(name string) http.HandlerFunc {
	p, ok := sc.UpstreamProxies[name]
	if !ok {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}
	}

	cb, hasBreaker := sc.Breakers[name]
	return func(w http.ResponseWriter, r *http.Request) {
		if !hasBreaker {
			p.ServeHTTP(w, r)
			return
		}

		err := cb.Do(func() error {
			p.ServeHTTP(w, r)
			return nil
		})
		if errors.Is(err, breaker.ErrServiceUnavailable) {
			http.Error(w, "circuit breaker open", http.StatusServiceUnavailable)
		}
	}
}
