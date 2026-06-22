# PRD — Video Upload & Streaming Platform

**Status:** Draft — assumptions below need validation before engineering scope is finalized.

This is a product requirements doc: what we're building and why. For how the existing services are engineered, see [internal-architecture.md](internal-architecture.md), [microservice.md](microservice.md), and [grpc.md](grpc.md). `MICROSERVICES_BLUEPRINT.md` and `README.md` at the repo root predate this PRD and describe generic Go-microservices practice (gateway, JWT, gRPC) independent of what product gets built on top — this doc is the product layer that now sits above that foundation.

## 1. Summary

A multi-tenant SaaS platform where organizations (tenants) upload, manage, and stream video content to their own users. Each tenant has its own user base and its own role-based permissions — tenants do not share content, users, or roles with each other.

## 2. Background

The platform started as a generic microservices reference build (`auth`, `user`, `catalog`, `docs`) — auth/JWT, gRPC between services, Kong gateway, database-per-service. That foundation now needs a real product on top of it. The chosen product: video upload and streaming, multi-tenant from day one.

## 3. Goals

- A tenant (organization) can sign up, invite members, and assign roles.
- A member with upload permission can upload a video; it becomes streamable without the uploader managing encoding/storage themselves.
- A member with view permission can browse and play back videos belonging to their tenant.
- Tenants are fully isolated: no tenant can see, list, or stream another tenant's content or users, even by guessing IDs.

## 4. Non-Goals (v1)

- Live streaming (RTMP/WebRTC ingest, low-latency delivery) — VOD (upload-then-watch) only.
- Public/anonymous video sharing outside a tenant.
- Monetization (subscriptions, pay-per-view, ads).
- Comments, likes, social features.
- Mobile native apps — responsive web only.
- Cross-tenant content sharing or marketplace features.

## 5. Assumptions

Flagging these explicitly since they shape scope significantly and weren't stated directly:

- **Shape of "tenant"**: a tenant is an organization with multiple internal members (think: a company's internal video library / training content / team recordings) — not an individual creator's public channel. This reading comes from "each tenant has its own users and RBAC," which implies multiple users per tenant with differentiated permissions, not a single-creator account.
- **Tenant resolution**: confirmed — email is globally unique across the whole platform, not per-tenant. A person belongs to exactly one tenant; there's no flow today for one email to hold separate accounts in two different orgs. Login stays `{email, password}`; tenant is resolved server-side from the user record.
- **Roles (v1)**: two roles per the existing roadmap — `admin` (manage tenant members, upload, delete any tenant video) and `member` (upload own videos, view all tenant videos). Anything more granular (per-video sharing, viewer-only role) is a v2 question.
- **Delivery**: adaptive-bitrate HLS, served from object storage behind a CDN — not progressive MP4 download.

## 6. Target Users

| Persona | Role | Needs |
|---|---|---|
| Tenant Admin | `admin` | Invite/remove members, assign roles, manage/delete any video in the tenant, see storage usage |
| Member | `member` | Upload own videos, browse and play all videos in the tenant |
| (v2) Viewer-only | — | Browse and play only — no upload. Deferred; needs a third role. |

## 7. Core User Flows

### 7.1 Tenant + first user signup
1. Org signs up → creates tenant + first user as `admin`.
2. Admin invites members by email; invited users set a password on first login.

### 7.2 Upload
1. Member selects a video file, starts upload.
2. Upload should be resumable/chunked — video files are large and uploads over flaky connections shouldn't restart from zero.
3. Video enters `processing` state immediately; uploader can leave the page.
4. Backend transcodes to multiple renditions (e.g., 1080p/720p/480p) for adaptive bitrate playback, generates a thumbnail.
5. Video moves to `ready` (playable) or `failed` (with a reason surfaced to the uploader).

### 7.3 Playback
1. Member browses the tenant's video library (list, search by title, filter by uploader/date).
2. Selects a video → adaptive-bitrate HLS playback, scrubbing/seeking supported once `ready`.
3. Playback respects tenant boundary: a signed-in member of tenant A can never retrieve a stream URL for tenant B's video, including by guessing/brute-forcing video IDs.

### 7.4 Management
1. Admin can delete any video in the tenant; member can delete their own uploads.
2. Admin can see aggregate storage used by the tenant.

## 8. Functional Requirements

| # | Requirement |
|---|---|
| FR1 | Tenant signup creates a tenant + first admin user |
| FR2 | Admin can invite, remove, and change the role of members within their own tenant only |
| FR3 | Upload accepts common formats (MP4, MOV, MKV at minimum) up to a configurable max size/duration |
| FR4 | Upload is resumable across connection drops |
| FR5 | Uploaded video is transcoded into at least 2 renditions for adaptive playback |
| FR6 | Video has a status: `uploading → processing → ready` or `failed` |
| FR7 | Playback URLs are tenant- and auth-scoped — not guessable, not valid for other tenants, expire after a bounded TTL |
| FR8 | Video list supports pagination, search by title, sort by upload date |
| FR9 | Deletion removes the video, its renditions, and its thumbnail from storage (not just the DB row) |
| FR10 | All of the above is enforced per-tenant: every query is implicitly scoped to the caller's `tenant_id` |

## 9. Non-Functional Requirements

- **Tenant isolation is the top security requirement.** A bug that leaks one tenant's video list or stream URL to another tenant is a sev-1, not a normal bug — every video/user query must be scoped by `tenant_id` derived from the authenticated session, never from a client-supplied value.
- **Upload resilience**: large files, unreliable networks — resumable upload, not a single atomic HTTP POST.
- **Processing is asynchronous**: transcoding must not hold an HTTP request open; the uploader gets an immediate response and polls/subscribes for status.
- **Storage cost awareness**: every video exists in original + N transcoded renditions; storage and transcoding both scale with usage and need to be tracked per tenant (ties to FR on storage usage visibility).
- **Streaming performance**: adaptive bitrate so playback degrades gracefully on poor connections instead of buffering.

## 10. Service Implications (high level)

Not a final architecture — flagging what this adds to the existing service map so engineering scoping can start from a realistic baseline:

- **`tenant`** (new) — tenant record, membership, invites. Likely the most foundational new piece: `auth` and `user` both need `tenant_id` before video work can start.
- **`user`** (existing, extends) — add `tenant_id` (FK), change the email-uniqueness model per §5, add `role`.
- **`auth`** (existing, extends) — JWT claims grow to `{uid, tenant_id, role}`; `auth.UserLookup`/the gRPC contract to `user` (`ExistsByEmailRequest`, `GetByEmailRequest`, `CreateRequest`) gains tenant context.
- **`video`** (new) — video metadata (title, status, owner, tenant), upload-session handling, library/search.
- **`transcoder`** (new, likely a worker, not a request-serving service) — consumes "video uploaded" events, produces renditions + thumbnail, updates status.
- Object storage (S3-compatible) + CDN — new infrastructure dependency, not currently in the stack.

`catalog` (the generic e-commerce "products" service from the original reference build) has been removed now that the product direction is set — it modeled a domain unrelated to video upload/streaming.

## 11. Success Metrics

- Time from "upload starts" to "video is `ready`" (processing latency).
- Upload success rate on the resumable path (vs. failures requiring a full restart).
- Zero cross-tenant data exposure incidents — this one has no acceptable nonzero number.
- Playback start time (time to first frame) and rebuffer rate.

## 12. Open Questions

These materially change scope depending on the answer — worth resolving before engineering breaks this into tickets:

1. **Is a tenant's content ever visible outside the tenant?** (e.g., a "share externally via link" feature) — assumed no for v1; confirm.
2. **Per-video access control beyond tenant-wide visibility?** — v1 assumes any member can see any video in their tenant. Folders/private uploads/viewer-restricted videos are a different (larger) feature.
3. **Self-serve signup vs. invite-only/sales-assisted onboarding?** — changes whether `tenant` needs billing/plan concepts in v1 or can defer them.
4. **Upload limits** — max file size, max duration, total storage quota per tenant (and what happens at the quota).
