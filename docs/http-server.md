# HTTP Server Configuration & Graceful Shutdown

## Server Timeouts

```go
srv := &http.Server{
    Addr:           ":" + strconv.Itoa(cfg.Port),
    Handler:        mux,
    ReadTimeout:    10 * time.Second,
    WriteTimeout:   10 * time.Second,
    IdleTimeout:    60 * time.Second,
    MaxHeaderBytes: 1 << 20, // 1 MiB
}
```

### ReadTimeout — 10s

Time allowed to read the **entire request** (headers + body). If a client connects but sends the request slowly (slow loris attack), the server closes the connection after 10 seconds.

Without this, a single slow client can hold a goroutine indefinitely. Go creates one goroutine per connection; unbounded slow connections exhaust memory.

### WriteTimeout — 10s

Time allowed to write the **entire response** after the request is received. Covers the handler execution time plus the time to send the response body.

10 seconds is generous for a JSON CRUD API where no handler should take more than a few hundred milliseconds. If a specific endpoint legitimately needs more time (e.g., a long-running export), it can be served on a separate server or the timeout can be extended for that route via `http.TimeoutHandler`.

### IdleTimeout — 60s

Time an idle keep-alive connection can remain open waiting for the next request. HTTP/1.1 clients reuse connections; 60 seconds is a reasonable window before the connection is reaped.

Without this, a mobile client that goes offline while keeping a TCP connection open would hold server resources indefinitely.

### MaxHeaderBytes — 1 MiB

Maximum size of request headers. Prevents header-based DoS (e.g., sending thousands of cookie headers). 1 MiB is far more than any legitimate client needs; typical headers are <8 KB.

Note: this is separate from the request body limit, which is enforced per-handler by `request.DecodeJSON` (also 1 MiB). Both limits exist independently.

## Graceful Shutdown

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
    slog.Error("server forced to shutdown", "error", err)
    os.Exit(1)
}
```

### Signal Handling

`SIGINT` — sent when a developer presses `Ctrl+C`.  
`SIGTERM` — sent by container orchestrators (Docker, Kubernetes) when stopping a pod.

Both signals trigger the same graceful shutdown path.

### `http.Server.Shutdown()`

`Shutdown()` stops accepting new connections and waits for in-flight requests to complete. It does **not** cancel in-flight request contexts — handlers run to completion.

The 15-second context timeout is the maximum time `Shutdown()` is allowed to wait. If in-flight requests don't complete within 15 seconds, the process exits anyway.

### Why 15 Seconds?

This service is behind a load balancer or Kubernetes service. The orchestrator:
1. Removes the pod from the endpoint list (stops sending new traffic) before sending SIGTERM
2. Waits for `terminationGracePeriodSeconds` (default 30s in k8s) before force-killing

The 15-second Go-level timeout covers the slowest realistic DB query under load (pagination counts on large tables, complex joins), giving in-flight requests a fair chance to complete. It is safely within Kubernetes' 30s window — the OS will never have to force-kill. The previous value of 1 second was too aggressive and would have dropped requests on every rolling deploy.

## Why `ListenAndServe` in a Goroutine?

```go
go func() {
    if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        slog.Error("server error", "error", err)
        os.Exit(1)
    }
}()
// main goroutine blocks on signal
<-quit
```

`ListenAndServe` blocks until the server stops. Running it in a goroutine frees the main goroutine to block on the signal channel. When the signal arrives, `Shutdown()` is called from the main goroutine, which causes `ListenAndServe` to return `http.ErrServerClosed` (not an error — it's the normal shutdown path).

## Alternatives Considered

- **No timeouts (default `http.Server`)** — Go's default `http.Server` has no timeouts set. Fine for internal tools; a DoS risk for any internet-facing API. Rejected.
- **`context.WithTimeout` per-request** — some frameworks apply a per-request deadline via middleware. Orthogonal to server-level timeouts; can be added on top if needed.
- **Longer graceful shutdown (30s)** — appropriate if handlers do long-running work. Not needed for CRUD; overly long shutdown delays rolling deployments.
- **`os.Exit(0)` on SIGTERM without graceful drain** — drops in-flight requests. Rejected; even 1 second of draining is better than none.
