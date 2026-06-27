# Async Messaging (RabbitMQ)

## Why

Registering a user triggers a side effect (today: a welcome email), but the registration request itself (`auth` → gRPC → `user`) must not block on, or fail because of, that side effect being slow or down. The fix is the same one [grpc.md](grpc.md)'s "Alternatives Considered" already named as deferred: publish a domain event after the write commits, and let an independent consumer react to it asynchronously.

`user` (which owns the row) publishes `user.created` after a successful insert; `notification` (a separate service) consumes it and acts on it — see [email.md](email.md) for what it actually does with the event. `auth` has no idea messaging exists at all — it only ever talks to `user` over gRPC, as before. This is the RabbitMQ half of [next.md](next.md) Phase 5; Kafka (event streaming, audit log) is a separate, still-unimplemented sub-phase, not needed for this use case — see Alternatives Considered.

## Topology

```
                    domain_events (topic exchange)
                            │ routing key: user.created
                            ▼
                 notification.user_created (queue)
                            │ Nack(requeue=false) on handler failure
                            ▼
        notification.user_created.retry.exchange (topic)
                            │ routing key: user.created
                            ▼
              notification.user_created.retry (queue)
                  x-message-ttl = NOTIFICATION_CONSUMER_RETRY_TTL
                            │ TTL expiry dead-letters back to domain_events
                            └──────────────► (back to the top)

  After NOTIFICATION_CONSUMER_MAX_RETRIES full cycles through the retry
  queue, the consumer stops retrying and republishes the message to:

              notification.user_created.dlq (queue)
                  — parked here for manual inspection, never auto-retried
```

`pkg/messaging/rabbitmq.DeclareConsumerTopology` declares all of this idempotently; both the publisher (`user`) and the consumer (`notification`) call it at startup, so neither has to assume the other started first. This is RabbitMQ's standard "retry via per-queue TTL + dead-letter-exchange bounce-back" pattern — used instead of the `rabbitmq-delayed-message-exchange` plugin (which RabbitMQ has no native equivalent of without a plugin) to keep one fewer moving part in the stack.

Retries are a **flat delay**, not an exponential backoff ladder: every retry waits the same `NOTIFICATION_CONSUMER_RETRY_TTL` before redelivery. A backoff ladder (multiple retry queues with increasing TTLs) is a reasonable future enhancement, not built now — see Known Gaps.

## Envelope

```go
// pkg/messaging/rabbitmq/envelope.go
type Envelope struct {
    EventID   string          `json:"event_id"`   // uuid, generated per publish — consumer dedup key
    EventType string          `json:"event_type"` // e.g. "user.created"
    Payload   json.RawMessage `json:"payload"`
    TraceID   string          `json:"trace_id"`    // reuses the X-Request-ID correlation convention
    Timestamp time.Time       `json:"timestamp"`
    Version   int             `json:"version"`
}
```

`TraceID` is populated from `middleware.GetRequestID(ctx)` — the same context value the HTTP and gRPC middleware chains already carry — so a registration request's logs and the async event it triggers can be grepped together by `request_id`, the same way [grpc.md](grpc.md)'s request ID propagation already ties `auth` and `user`'s logs together.

Event contracts (the `Payload` shape for a given `EventType`) live in `pkg/events/<producer>/` — e.g. `pkg/events/user/` holds `UserCreated` (the event_type constant) and `UserCreatedPayload`. This mirrors how `pkg/proto/<service>/` already shares gRPC contracts between `auth` and `user`: one subpackage per producer, so a second event type or producer doesn't grow a single flat file.

## Publisher — `user` service

`services/user/internal/platform/messaging/publisher.go` adapts `pkg/messaging/rabbitmq.Publisher` to the `user.EventPublisher` interface `UserService` depends on:

```go
type EventPublisher interface {
    PublishUserCreated(ctx context.Context, u *User) error
}
```

`UserService` has a single private `create()` helper that both the HTTP `Create` and the gRPC `CreateWithPassword` path call — insert, then publish:

```go
func (s *UserService) create(ctx context.Context, u *User) (*User, error) {
    created, err := s.repo.Create(ctx, u)
    if err != nil {
        return nil, err
    }
    if err := s.publisher.PublishUserCreated(ctx, created); err != nil {
        logger.FromContext(ctx).Error("publishing user.created event", "error", err, "user_id", created.ID)
    }
    return created, nil
}
```

The publish happens strictly **after** `repo.Create` returns successfully — never inside a transaction — and a publish failure is logged but never fails the request: the user row is the source of truth, and a lost event is recoverable in a way a failed registration is not. See Known Gaps for exactly what this trades away.

Until this change, the gRPC `Create` RPC (the path `auth.Register` actually calls) bypassed `UserService` entirely and wrote straight to the repository — so no service-layer hook could ever have fired on a real registration. `GRPCServer.Create` now calls `UserService.CreateWithPassword`, which funnels through the same `create()` helper as the HTTP path, fixing that gap as part of wiring in publishing.

`Publisher.Publish` uses confirm mode (`Channel.Confirm` + `PublishWithDeferredConfirmWithContext`) — it only returns success once the broker has acknowledged the message, not merely once it left the client's TCP buffer.

## Consumer — `notification` service

`pkg/messaging/rabbitmq.Consumer.Consume` runs with manual acknowledgement:

- **Success** → `Ack`.
- **Failure**, retries remaining (read from the delivery's `x-death` header — see `retriesSoFar` in `consumer.go`) → `Nack(requeue=false)`, which the topology above bounces through the retry queue.
- **Failure**, retries exhausted → republish the raw message to `notification.user_created.dlq`, invoke the `OnExhausted` callback (so the caller can record the terminal failure), then `Ack` the original. A permanently failing message is never retried forever and never silently dropped.

The `Handler` and `OnExhausted` functions the consumer invokes are plain callbacks — this package has no idea what a "notification" or an "email" is, only that some event-processing function either succeeded or didn't. `services/notification/cmd/api/main.go` wires `Consumer.Consume` to `NotificationService.HandleUserCreated` (and `OnExhausted` to `NotificationService.RecordFailure`); what those actually do — render a template, send mail, write an audit row — lives in [email.md](email.md), not here.

`HandleUserCreated` is idempotent at the messaging layer regardless of what its side effect is: it checks `ExistsByEventID` before doing anything, so a redelivery of an event that already produced a row is a no-op:

```go
func (s *NotificationService) HandleUserCreated(ctx context.Context, eventID string, payload eventsuser.UserCreatedPayload) error {
    if exists, err := s.repo.ExistsByEventID(ctx, eventID); err != nil {
        return err
    } else if exists {
        return nil // already processed
    }
    // perform the side effect, then repo.Create(status=sent)
}
```

The `ExistsByEventID` check is a fast-path optimization, not the actual enforcement — `notifications.event_id` has a `UNIQUE` constraint, and `Create` returning a unique-violation (the same `23505`/`apperror.Conflict` idiom `services/user/internal/user/repository.go` already uses) is treated as "already processed" too. This closes the race where two redeliveries of the same event are being processed concurrently by different consumer instances.

## Configuration

| Env var | Service | Meaning |
|---|---|---|
| `RABBITMQ_URL` | user, notification | AMQP URI, e.g. `amqp://app:pass@rabbitmq:5672/` |
| `RABBITMQ_EXCHANGE` | user, notification | Topic exchange events are published to (default `domain_events`) |
| `NOTIFICATION_CONSUMER_MAX_RETRIES` | notification | Retry cycles before parking in the DLQ (default `3`) |
| `NOTIFICATION_CONSUMER_RETRY_TTL` | notification | Flat delay per retry cycle (default `30s`) |

The username for the shared RabbitMQ container is deliberately not `guest` — RabbitMQ hardcodes the `guest` user to loopback-only connections, which would block every other container on the Docker network from authenticating.

## Known Gaps

- **At-most-once publish, not exactly-once — no outbox pattern.** The publish happens in-process, after the DB commit, best-effort. If the process crashes between the DB commit and the publish call succeeding, the event is silently lost with no automatic recovery. No `outbox` table, no reconciliation job. This is acceptable for the current event (a missed welcome email is a UX gap, not a financial or safety-critical failure) — it would not be acceptable for, say, a payment-confirmation event. A transactional outbox (write the event to a DB table in the same transaction as the business write, relay it separately) is the standard fix if a future event type needs stronger guarantees.
- **No automatic reconnect.** `pkg/messaging/rabbitmq.Dial` connects once at startup and fails fast if unreachable, the same maturity level as every other platform dependency in this repo (see [grpc.md](grpc.md)'s Known Gaps on the `auth`→`user` gRPC connection). A RabbitMQ restart currently requires restarting the dependent services too.
- **Single RabbitMQ node — no clustering/HA.** Fine for local Docker Compose; a production deployment would need a clustered broker (or a managed one) so a single node failure doesn't stop all event flow.
- **Flat-delay retry, no exponential backoff ladder.** Every retry waits the same fixed TTL. A backoff ladder (multiple retry queues with increasing TTLs) would reduce load on a struggling downstream dependency better, but adds more topology than this use case currently justifies.
- **DLQ-parking is two non-atomic steps.** `Consumer.park` republishes to the DLQ queue, then acks the original — a crash between those two steps can produce a duplicate DLQ entry. Whether that's harmless depends entirely on the consumer's own idempotency — `notification`'s is (see [email.md](email.md)), but this package can't guarantee that for an arbitrary future consumer.
- **No schema registry or versioning beyond `Envelope.Version`.** A breaking payload change for an existing `event_type` has no migration story yet — fine with a single producer and a single consumer, would need attention before a second consumer depends on the same event.

## Alternatives Considered

- **Outbox pattern instead of best-effort publish** — stronger delivery guarantee, at the cost of an `outbox` table in `user_db` plus a relay/poller process. Deferred: the current event tolerates the small at-most-once gap described above; revisit if a future event type doesn't.
- **Kafka instead of / in addition to RabbitMQ** — better fit for event streaming, replay, and audit logs (see [next.md](next.md) Phase 5's Kafka sub-phase), worse fit for this use case: a single task ("react once per registration") with no need for replay or multi-consumer fan-out. RabbitMQ's task-queue model — and its DLQ-on-exhaustion behavior — matches this directly.
- **Exponential backoff ladder for retries** — more resilient under sustained downstream failure, more topology to declare and reason about. A flat-delay retry was judged sufficient for a brief "consumer is temporarily down" failure mode; revisit if retries start exhausting under real load.
- **Redis-based idempotency dedup instead of a DB unique constraint** — would need its own TTL-eviction policy and adds a second source of truth. The DB already enforces uniqueness transactionally and doubles as the audit trail, so it was the simpler choice.
