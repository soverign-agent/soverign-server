// Package middleware provides HTTP middleware for the API gateway.
package middleware

import "net/http"

// Middleware is a standard HTTP middleware signature.
type Middleware func(http.Handler) http.Handler

// Chain composes multiple Middlewares into a single wrapper.
// Middlewares are applied left-to-right.
func Chain(mws ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}

// Adapt converts a go-zero-style http.HandlerFunc middleware into a standard Middleware.
func Adapt(mw func(http.HandlerFunc) http.HandlerFunc) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(mw(next.ServeHTTP))
	}
}
