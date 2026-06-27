# Email

## Why

`notification` needs to send a welcome email when it consumes `user.created` (see [messaging.md](messaging.md) for how it receives that event — this doc is only about what it does once it has it: render and send the email, then record what happened). The transport (`pkg/mailer`) is shared infrastructure with no business logic in it, the same boundary every other `pkg/` package draws — template content and when-to-send decisions live in `services/notification`.

## `pkg/mailer`

A one-method interface, so the notification service depends on an abstraction, not a concrete SMTP client:

```go
// pkg/mailer/mailer.go
type Message struct {
    To      string
    Subject string
    Body    string // pre-rendered HTML — mailer is transport only
}

type Sender interface {
    Send(ctx context.Context, msg Message) error
}
```

`SMTPSender` (`pkg/mailer/smtp.go`) is the only implementation today, built on the standard library's `net/smtp` — no third-party SMTP dependency. `Send` does the full handshake by hand:

1. `(&net.Dialer{}).DialContext(ctx, "tcp", host:port)` — a context-aware dial, so the request's deadline/cancellation actually bounds the SMTP call.
2. `smtp.NewClient` wraps the connection.
3. If `NOTIFICATION_SMTP_USE_TLS=true` and the server advertises the `STARTTLS` extension, upgrades the connection (`tls.Config{ServerName: host}`).
4. If a username is configured and the server advertises `AUTH`, authenticates with `smtp.PlainAuth`.
5. `MAIL` → `RCPT` → `DATA`, writing a hand-built MIME message (`From`/`To`/`Subject` headers, `Content-Type: text/html; charset="UTF-8"`, blank line, then the rendered body) as the payload.
6. `QUIT`.

Pointing this at a real provider (SES, SendGrid, Mailgun, ...) in production vs. [Mailpit](https://github.com/axllent/mailpit) locally is purely a config change — same code path either way, only `NOTIFICATION_SMTP_*` differs.

## Template — `services/notification`

The welcome email's HTML lives in `services/notification/internal/notification/templates/welcome.html`, embedded at compile time the same way `services/docs/docs.go` embeds its static assets:

```go
// services/notification/internal/notification/templates.go
//go:embed templates/*.html
var templatesFS embed.FS

var welcomeTemplate = template.Must(template.ParseFS(templatesFS, "templates/welcome.html"))
```

`html/template` (not `text/template`) is used deliberately — it auto-escapes values interpolated into the template (e.g. a user's `Name`), so a name containing `<script>` can't inject HTML into the rendered email.

Template *content* is notification's business logic, not generic transport, which is why it lives here rather than in `pkg/mailer` — a future second email (password reset, etc.) gets its own `.html` file in the same directory, not a change to `pkg/mailer`.

## Sending flow

`NotificationService.HandleUserCreated` (`services/notification/internal/notification/service.go`) is what actually triggers a send, after the idempotency check described in [messaging.md](messaging.md):

```go
body, err := renderWelcomeEmail(payload.Name)
// ...
err = s.sender.Send(ctx, mailer.Message{
    To:      payload.Email,
    Subject: "Welcome!",
    Body:    body,
})
```

A send failure is returned untouched, not retried internally — [messaging.md](messaging.md)'s consumer retry/DLQ logic decides whether to redeliver. This keeps `pkg/mailer` and the service layer simple: neither needs its own retry/backoff, since the messaging layer already provides one.

## Audit trail — `GET /v1/notifications`

Only on a successful send does `HandleUserCreated` write a `notifications` row (`status=sent`). If retries are exhausted without a successful send, `cmd/api/main.go`'s `OnExhausted` callback calls `NotificationService.RecordFailure` instead, writing a `status=failed` row with the last error — so a permanently failing send is still visible via the read API even though no email ever went out for it:

```
GET /v1/notifications/           # paginated, filterable by recipient/type/status
GET /v1/notifications/{id}
```

This is read-only by design — rows are written only by the consumer, never by a client request.

## Configuration

| Env var | Meaning |
|---|---|
| `NOTIFICATION_SMTP_HOST` / `_PORT` | SMTP server address — `mailpit:1025` locally |
| `NOTIFICATION_SMTP_USERNAME` / `_PASSWORD` | SMTP auth — empty for Mailpit, required for most real providers |
| `NOTIFICATION_SMTP_FROM_NAME` / `_FROM_ADDRESS` | `From:` header on outgoing mail |
| `NOTIFICATION_SMTP_USE_TLS` | STARTTLS — `false` for Mailpit, `true` for real providers |

Locally, Mailpit's web UI (`http://localhost:8025`) shows every email `notification_app` sends — nothing leaves the Docker network.

## Known Gaps

- **No bounce/delivery-status handling.** `Send` returning without error only means the SMTP server accepted the message for delivery — it says nothing about whether the recipient's mailbox actually received it. A real provider's bounce webhooks (or polling its API) would be needed to know that, and nothing here consumes them.
- **No HTML/plaintext multipart fallback.** The message is `Content-Type: text/html` only — some mail clients prefer a `multipart/alternative` with a plaintext part. Not implemented; low risk for a short welcome message.
- **No per-provider rate-limit awareness.** A real SMTP provider may throttle or temporarily reject under burst load; `SMTPSender` doesn't distinguish that from any other failure, so it just flows into the generic retry/DLQ path in [messaging.md](messaging.md) rather than backing off specifically for that case.
- **The read API is JWT-gated like `/v1/users`, not admin-restricted.** Any authenticated user can currently list all sent notifications via `GET /v1/notifications`, including other users' email addresses. Consistent with the rest of the codebase's current security posture — there's no RBAC anywhere yet ([next.md](next.md) Phase 1 is unchecked) — but should move to an admin-only role once RBAC exists.

## Alternatives Considered

- **A provider SDK (SES/SendGrid Go client) instead of raw SMTP** — often simpler and gets provider-specific features (templates, analytics, bounce webhooks) for free. Rejected for now: it would tie `pkg/mailer` to one provider's API shape, whereas every provider also speaks plain SMTP, so the generic `net/smtp` path works against all of them (including Mailpit locally) with zero provider-specific code. Revisit if a provider's webhook/analytics features become worth the coupling.
- **Sending the email synchronously inside `auth.Register`** — simplest, no broker needed. Rejected: ties registration latency/availability to SMTP, and is the exact problem [messaging.md](messaging.md) exists to avoid.
- **A generic `Notify(event)` interface instead of a `NotificationService`-specific `HandleUserCreated`** — more "pluggable" for hypothetical future event types. Not built: with exactly one event type and one consumer, a generic dispatch layer would be speculative; add it when a second event type actually needs handling.
