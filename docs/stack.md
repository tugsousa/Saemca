# Technology Stack — SALUSDOMI

This document records every technology decision, the alternatives considered, and the reason for each choice.
Future developers: do not change a tool without understanding why it was chosen.

---

## Backend

### Language: Go 1.23+
**Chosen because:** High concurrency (critical for WebSocket connections and geo-broadcasting), strong type system, fast compilation, single binary deployment — ideal for a marketplace that will handle simultaneous booking requests.

### HTTP Router: Chi
**Chosen because:** Lightweight, idiomatic Go, middleware composable, no magic. Small enough to understand fully; powerful enough for production.
**Alternatives considered:** Gin (more magic, less idiomatic), Fiber (fast but non-standard net/http), Echo (fine, but Chi is simpler).

### Database: PostgreSQL 18 + PostGIS
**Chosen because:** PostGIS is the only production-grade solution for the geography queries this app requires (`ST_DWithin` for proximity search, `ST_Within` for arrival detection). No alternative considered — PostGIS was a hard requirement.

### DB Migration Tool: golang-migrate
**Chosen because:** Plain SQL files, sequential versioning, works with the existing `/infra/migrations/` structure.
**Rule:** Migration files are numbered `000001_description.sql`, `000002_description.sql`. Never edit an existing file — always create a new one.

### Authentication
**Pattern:** JWT access token (15-min expiry) + Refresh Token (30-day, httpOnly cookie, hashed in DB)
**Libraries:**
- JWT: `golang-jwt/jwt/v5`
- Passwords: `golang.org/x/crypto/argon2` (Argon2id — current gold standard for password hashing)

**Why this pattern:** Short-lived JWT means stolen tokens expire quickly. Refresh token in httpOnly cookie is inaccessible to JavaScript (XSS protection). Storing the refresh token in the DB means sessions can be revoked instantly (critical for a healthcare platform).

**Why NOT a single long-lived JWT:** Cannot revoke. If a user's account is compromised, you cannot force-logout them until the token expires.

### File Storage: Cloudflare R2 (prod) + MinIO (dev)
**Chosen because:** R2 has zero egress fees (pay to store, not to serve), S3-compatible API, works regardless of which VPS provider is used (Hetzner or OVH). MinIO in development is API-identical — same code, different env vars.
**Alternatives considered:** AWS S3 (egress costs add up), Hetzner Object Storage (fine but ties you to Hetzner), OVH Object Storage (fine but ties you to OVH).
**Go library:** `aws-sdk-go-v2/service/s3` (works with any S3-compatible storage)

### Email: Resend
**Chosen because:** Modern developer experience, reliable delivery, simple API, generous free tier, works well in EU.
**Used for:** Booking confirmations, receipts, account verification, password reset.

### SMS: Vonage (formerly Nexmo)
**Chosen because:** Strong EU coverage, competitive Portugal pricing, mature Go SDK.
**Used for:** "Nurse en route" alerts, booking confirmation for clients without browser notifications.
**Alternatives considered:** Twilio (more expensive for EU), AWS SNS (more complex setup).

### Payments: Stripe Connect
**Non-negotiable:** Only payment processor that supports MB-WAY (Portugal's dominant payment method) AND automated marketplace payouts to professionals.
**Pattern:** Funds are authorized at booking, captured only after double-handshake confirmation. Disputed bookings freeze funds until Admin resolves.

### Logging: slog (stdlib)
**Chosen because:** Built into Go 1.21+ standard library, zero dependencies, JSON output, structured key-value pairs.
**Configuration:** JSON handler, output to stdout. Docker captures stdout and ships to log aggregator.
**Why not zerolog/zap:** Adds a dependency for something the stdlib now handles correctly.

### Error Tracking: Sentry
**Chosen because:** Captures full stack traces with request context in production. Both Go backend SDK and React frontend SDK available. Critical for a solo developer — you need to know what broke before users report it.

### Log Aggregation: Better Stack (Logtail)
**Chosen because:** Accepts JSON logs via HTTP endpoint, generous free tier, clean UI for search and alerting.

### Metrics: Grafana Cloud (free tier)
**Chosen because:** Connect a lightweight agent to the VPS, get CPU/memory/disk/request metrics without self-hosting a Prometheus stack.

---

## Frontend

### Framework: React 19 + Vite
**Chosen because:** Vite's fast HMR dramatically improves development speed. React 19's concurrent features improve perceived performance for the booking flow.

### Routing: TanStack Router
**Chosen because:** Fully type-safe routes and parameters (TypeScript errors if you navigate to a non-existent route or pass wrong params). Code-splitting per route is built-in. Better developer experience than React Router v6.
**Alternatives considered:** React Router v6 (less type-safe), Next.js (overkill for this SPA use case, adds SSR complexity we don't need).

### Server State: TanStack Query
**Chosen because:** Handles all data-fetching concerns automatically: caching, background refetching, loading/error states, pagination, optimistic updates. Eliminates 80% of useEffect + useState boilerplate for API calls.

### Client State: Zustand
**Chosen because:** Simple, minimal boilerplate. Used only for UI state that doesn't come from the server (e.g., which profile is currently selected, modal open/closed state).
**Not used for:** API data — that belongs in TanStack Query.

### Components: shadcn/ui
**Chosen because:** Built on Radix UI (accessible primitives) + Tailwind. When you add a component, the source code is copied into your project — you own it, it's not a runtime dependency. Fully customizable. Production-quality accessibility out of the box.
**Alternatives considered:** MUI (too opinionated, hard to style), Mantine (fine, but runtime dependency), Headless UI (less complete than Radix).

### Forms: React Hook Form + Zod
**Chosen because:** React Hook Form has the best performance (uncontrolled inputs, minimal re-renders). Zod provides schema-based validation that is shared between frontend forms and backend API validation.

---

## Infrastructure

### Containerization: Docker + Docker Compose
**For development:** `docker-compose.yml` in `/infra/` spins up PostgreSQL + PostGIS + MinIO.
**For production:** Each service (API, worker, nginx) runs as a Docker container on the VPS.

### Reverse Proxy: Nginx
**Handles:** SSL termination, routing requests to Go API, serving static frontend assets.

### CDN / DDoS Protection: Cloudflare
**Handles:** Static asset caching, DDoS mitigation, DNS management. Free tier is sufficient for MVP scale.

### VPS: Hetzner or OVH
**Requirement:** Must be EU-hosted (GDPR compliance — health data cannot leave the EU).
**Hetzner preferred** if undecided — better price/performance ratio, simple UI, Hetzner Object Storage available as backup option.

---

## Testing

### Unit Tests: Go stdlib `testing` + testify
- `github.com/stretchr/testify` for clean `assert`/`require` helpers
- Tests the service layer in isolation (pure business logic)

### Integration Tests: testcontainers-go
- `github.com/testcontainers/testcontainers-go`
- Spins up a real PostgreSQL + PostGIS Docker container per test run
- Tests the repository layer against a real database
- **Rule: never mock the database** — mock/prod divergence has caused real incidents

### End-to-End Tests: Playwright (TypeScript)
- Full browser → frontend → backend → database flows
- Located in `/e2e/` at repo root
- Covers all critical paths: login, booking flow, payment, material checklist, dispute

---

## Decisions Deferred to Later

| Feature | Reason Deferred | When to Revisit |
|---|---|---|
| Real-time GPS broadcasting (ASAP) | Browser GPS unreliable for continuous tracking | When native mobile app is built |
| Mobile App (React Native) | Web-first MVP | After web app is stable |
| Browser Push Notifications | Requires Service Workers + SSL setup | Post-launch, add as enhancement |
| 2FA / MFA | Not in initial scope | If enterprise clients or regulatory requirement |
| Microservices | Modular monolith is correct for current scale | Only if specific service needs independent scaling |
