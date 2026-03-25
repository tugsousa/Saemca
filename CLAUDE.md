# CLAUDE.md — AI Assistant Context for SALUSDOMI

This file is read automatically by Claude Code at the start of every session.
It contains the project context, rules, and conventions that must be followed consistently.

---

## Project Overview

**SALUSDOMI** (SaúdeEmCasa) is an On-Demand Home Healthcare Marketplace for Portugal.
It connects families with licensed healthcare professionals (nurses, physiotherapists, etc.)
for home visits. Think Uber for home healthcare.

**Three user roles:** CLIENT (family/patient), PRO (professional), ADMIN (platform operator).
**Platform country:** Portugal. Language: Portuguese (pt-PT). Currency: EUR (cents in DB).
**Legal sensitivity:** Handles personal health data — GDPR compliance is non-negotiable.

---

## Monorepo Structure

```
/SALUSDOMI
├── /backend        # Go API + Worker (Cron Jobs)
├── /frontend       # React Web App (3 portals: client, pro, admin)
├── /infra          # Docker Compose, Nginx, Migrations
├── /docs           # Technical docs and user stories
├── CLAUDE.md       # This file
└── README.md       # Entry point
```

---

## Technology Stack

### Backend
- **Language:** Go 1.23+
- **Router:** Chi (`github.com/go-chi/chi/v5`)
- **Database:** PostgreSQL 18 + PostGIS extension
- **DB Migrations:** `golang-migrate/migrate`
- **Auth:** JWT (access) + Refresh Tokens (httpOnly cookie, DB-backed)
  - JWT library: `golang-jwt/jwt/v5`
  - Password hashing: `golang.org/x/crypto/argon2` (Argon2id)
- **Logging:** `slog` (stdlib) with JSON handler — no third-party logger
- **File Storage:** S3-compatible — MinIO (dev), Cloudflare R2 (prod)
  - Client library: `aws-sdk-go-v2/service/s3`
- **Email:** Resend
- **SMS:** Vonage
- **Payments:** Stripe Connect (MB-WAY + Pro payouts)
- **Error Tracking:** Sentry (`github.com/getsentry/sentry-go`)
- **Go Module:** `salusdomi.com/api`

### Frontend
- **Framework:** React 19 + Vite
- **Styling:** TailwindCSS
- **Routing:** TanStack Router (type-safe)
- **Server State:** TanStack Query (data fetching, caching)
- **Client State:** Zustand (UI state only)
- **Components:** shadcn/ui (Radix UI + Tailwind — code is owned, not a runtime dependency)
- **Forms:** React Hook Form + Zod
- **Error Tracking:** Sentry (`@sentry/react`)

### Infrastructure
- **Containers:** Docker + Docker Compose
- **Reverse Proxy:** Nginx (SSL termination)
- **CDN / DDoS:** Cloudflare
- **VPS:** Hetzner or OVH (EU-based, GDPR compliant)
- **Object Storage:** Cloudflare R2

---

## Backend Architecture: Modular Monolith (DDD)

The backend follows Domain-Driven Design. Each domain is self-contained.

### Layer Structure (per domain)
```
/internal/{domain}/
├── handler.go      # HTTP layer only — no business logic here
├── service.go      # Business logic — no DB calls here
├── repository.go   # DB queries only — no business logic here
└── models.go       # Domain-specific types (if needed beyond /domain/models.go)
```

### Layer Rules
- `handler.go` → calls `service.go` only. Parses request, returns response.
- `service.go` → contains all business rules. Calls `repository.go` and other services.
- `repository.go` → contains all SQL. Returns domain types, not raw DB rows.
- Never skip layers. A handler must not call a repository directly.
- Never put HTTP concerns (status codes, request parsing) in the service layer.

### Domain Build Order
Build domains in this order — each must be fully working and tested before the next:
1. `auth` — JWT issuance, refresh tokens, login/logout
2. `user` — User and Profile management
3. `catalog` — Service types, pricing, materials
4. `booking` — Core engine, matching, state machine
5. `payment` — Stripe Connect, escrow, payouts
6. `geo` — PostGIS queries, proximity search
7. `chat` — WebSocket, message persistence
8. `worker` — Cron jobs (cancellation guard, payout trigger)

---

## API Contract

Every API response uses this standard wrapper — no exceptions:

```json
{
  "trace_id": "req-uuid",
  "success": true,
  "data": {},
  "meta": { "page": 1, "total": 50 },
  "error": {
    "code": "ERROR_CODE_SNAKE_UPPER",
    "message": "Human-readable message.",
    "trace_id": "req-uuid"
  }
}
```

- `success: true` → `data` is populated, `error` is null
- `success: false` → `error` is populated, `data` is null
- `meta` is only present on paginated responses
- Error codes are SCREAMING_SNAKE_CASE strings (e.g., `PROFILE_NOT_FOUND`)

---

## Authentication Rules

- Access Token: JWT, short-lived (15 minutes), returned in response body
- Refresh Token: Opaque UUID, stored as hash in `refresh_tokens` DB table, sent as `httpOnly` cookie (30-day expiry)
- **Never store JWT in localStorage** — always in JS memory only
- **Never store raw refresh tokens** — always hash before saving to DB
- Every protected endpoint validates the JWT via middleware
- Profile ownership is verified on every request via `Profile-ID` header middleware (IDOR protection)

---

## Database Rules

- All IDs are UUID (`gen_random_uuid()`)
- All timestamps use `TIMESTAMP WITH TIME ZONE`
- Monetary values stored as integers in **cents** (e.g., 1000 = €10.00)
- Migrations live in `/infra/migrations/` and are numbered sequentially (`000001_`, `000002_`, etc.)
- Never modify an existing migration — always create a new one
- Health-sensitive fields (`notes` in profiles) must be marked for encryption at rest
- The `materials_snapshot` in bookings is immutable after creation (legal record)

### Complete Table List
- `users` — identity + role
- `refresh_tokens` — auth session management
- `profiles` — patient data (MANAGED or INDEPENDENT)
- `professionals` — pro-specific data, location, rating
- `catalog` — service types, pricing, duration, materials required
- `materials` — checklist items per catalog service
- `bookings` — core transaction + state machine
- `reviews` — post-booking ratings (1-5 stars)
- `documents` — pro onboarding uploads (Cédula, insurance)
- `audit_logs` — GDPR access log (every read/write of health data)
- `fee_tiers` — platform fee rules (e.g., Founding 50 = 0%)
- `disputes` — dispute records with resolution + appeal tracking
- `dispute_evidence` — statements and file attachments per party
- `dispute_messages` — private Admin ↔ party threads during dispute

---

## Dispute Domain Rules

**Full spec:** [docs/disputes.md](docs/disputes.md) — read this before implementing anything in the dispute domain.

**Key rules for implementation:**
- Both CLIENT and PRO can open a dispute on a `COMPLETED_PRO` booking
- When a dispute opens: booking chat → READ-ONLY immediately (no new messages)
- A separate `dispute_messages` thread opens per party (Admin ↔ Client, Admin ↔ Pro) — parties cannot see each other's thread
- Payment stays `AUTHORIZED` (frozen) for the entire dispute lifecycle — never capture or release without an explicit resolution action
- Mutual resolution requires **both parties to click "Accept"** via in-app button — do not auto-resolve on admin proposal alone
- Appeals require **new evidence** — reject silently if no new evidence is attached
- The appeal must be assigned to a **different Admin** than the original resolver
- Write every Admin action (resolution, flag, request-more-info) to `audit_logs`
- `disputes_lost` counter on `users` increments only when a dispute resolves **against** that user — not when they open one
- Flag threshold is controlled by `DISPUTE_FLAG_THRESHOLD` env var (default: 3) — never hardcode it

---

## Testing Rules

### Unit Tests
- Location: `_test.go` files alongside the code they test
- Test service-layer business logic in isolation
- Use `github.com/stretchr/testify/assert` and `require`
- No database, no network, no external dependencies

### Integration Tests
- Use `github.com/testcontainers/testcontainers-go` — spins up real PostgreSQL + PostGIS
- Test repository layer against a real database — **never mock the database**
- Each test gets a clean DB state (transaction rollback or truncate)

### End-to-End Tests
- Use Playwright (TypeScript)
- Located in `/e2e/` at the repo root
- Test critical user flows end-to-end: booking creation, login, payment, material checklist
- Run against a local full-stack Docker environment

### Commands
```
make test-unit          # fast, no Docker
make test-integration   # requires Docker
make test-e2e           # requires full stack running
```

---

## Frontend Rules

- Business logic lives in **custom hooks** (`useBooking`, `useProfile`, etc.) — not in components
- Components are UI-only: they render data from hooks, emit events back to hooks
- This separation allows future React Native reuse of hook logic
- Zod schemas are the single source of truth for form validation shapes
- API calls go through TanStack Query only — never raw `fetch` in components

---

## Security Rules (Non-Negotiable)

- All inputs validated and sanitized before DB queries (use parameterized queries always)
- No SQL string concatenation — ever
- CORS configured to allow only known origins
- Rate limiting on all auth endpoints (`/auth/login`, `/auth/refresh`)
- File uploads: validate MIME type server-side, not just extension. Virus scan before storing.
- GDPR: every read/write of a `profiles.notes` or health-related field must write to `audit_logs`
- `Right to be Forgotten`: account deletion triggers `data_purger.go` — hard-deletes health data, retains transaction IDs for 10 years (Portuguese fiscal law)

---

## Observability Stack

- **Structured Logging:** `slog` with JSON output → stdout → Docker captures → shipped to Better Stack (Logtail)
- **Error Tracking:** Sentry (both backend Go SDK and frontend React SDK)
- **Metrics / Uptime:** Grafana Cloud free tier (connected to VPS agent)
- Log every significant state change (booking status transitions, payment events, profile access)
- Log format: always include `trace_id`, `user_id` (when available), `domain`, `action`

---

## Configuration & Secrets Rules

- All config is loaded once at startup via `internal/config/config.go`
- Never call `os.Getenv()` directly in business logic — always use the `Config` struct
- Never log secret values — `Config.LogValue()` is the only safe way to log config state
- Required fields in `Config` use `required:"true"` — the app refuses to start if missing
- `.env` files are gitignored. `.env.example` is the only secrets-related file in git
- In development: `.env` file loaded automatically. In production: real env vars on the server
- When adding a new config value: add to `Config` struct + add to `.env.example` with a comment

## What NOT to Do

- Do not add error handling or fallbacks for impossible scenarios
- Do not mock the database in integration tests
- Do not store JWTs in localStorage
- Do not concatenate strings in SQL queries
- Do not put business logic in `handler.go`
- Do not put HTTP concerns in `service.go`
- Do not skip the layer architecture (handler → service → repository)
- Do not modify existing migration files — always add a new one
- Do not store monetary values as floats — always use integer cents
- Do not write comments that just restate what the code does — only comment the "why"
- Do not add features or abstractions beyond what is currently needed
