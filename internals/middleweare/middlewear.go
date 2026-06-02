package middleweare

import "net/http"

type Middleware func(http.Handler) http.Handler

func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// With wraps a single handler func with route-specific middleware, running
// them in the order given. Use it to attach middleware to one route instead
// of the whole mux, e.g. mux.Handle("POST /", With(h.Create, Test)).
func With(h http.HandlerFunc, middlewares ...Middleware) http.Handler {
	return Chain(middlewares...)(h)
}
