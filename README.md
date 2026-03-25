# SALUSDOMI — SaúdeEmCasa

**On-Demand Home Healthcare Marketplace for Portugal.**

Connects families with licensed healthcare professionals (nurses, physiotherapists, etc.) for home visits. Handles booking, payments, professional verification, and dispute resolution.

---

## Documentation

| Document | Description |
|---|---|
| [CLAUDE.md](CLAUDE.md) | AI assistant context — conventions, rules, architecture (read before coding) |
| [docs/stack.md](docs/stack.md) | Every tool choice with rationale and alternatives considered |
| [docs/development.md](docs/development.md) | Local setup, git workflow, coding conventions, testing |
| [docs/technical_design.md](docs/technical_design.md) | System architecture, API contract, component definitions |
| [docs/user_stories.md](docs/user_stories.md) | Full user stories across all 7 epics |

**New to the project? Read in this order:**
1. This README
2. [docs/technical_design.md](docs/technical_design.md) — understand what we're building
3. [docs/stack.md](docs/stack.md) — understand how we're building it
4. [docs/development.md](docs/development.md) — get your environment running

---

## Architecture

**Tri-App Frontend** (Client, Professional, Admin portals) backed by a **Go Modular Monolith** following Domain-Driven Design.

```
/SALUSDOMI
├── /backend        # Go API + Worker (Cron Jobs)
├── /frontend       # React Web App (3 portals)
├── /infra          # Docker, Nginx, DB Migrations
├── /e2e            # Playwright end-to-end tests
└── /docs           # All technical documentation
```

### Core Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.23+ + Chi Router |
| Database | PostgreSQL 18 + PostGIS |
| Frontend | React 19 + Vite + TailwindCSS |
| Routing | TanStack Router |
| State | TanStack Query + Zustand |
| Components | shadcn/ui |
| Payments | Stripe Connect (MB-WAY + payouts) |
| Storage | Cloudflare R2 (MinIO in dev) |
| Auth | JWT + Refresh Tokens (httpOnly cookie) |
| Email | Resend |
| SMS | Vonage |
| Errors | Sentry (backend + frontend) |
| Infra | Docker + Nginx + Cloudflare + Hetzner/OVH |

---

## Quick Start

```bash
# 1. Start infrastructure
cd infra && cp .env.example .env && docker compose up -d

# 2. Run migrations
make migrate-up

# 3. Start backend
cd backend && go run ./cmd/api

# 4. Start frontend
cd frontend && npm install && npm run dev
```

Full setup instructions: [docs/development.md](docs/development.md)

---

## Three User Roles

| Role | Portal | Description |
|---|---|---|
| **CLIENT** | `/apps/client` | Families booking care for themselves or dependents |
| **PRO** | `/apps/pro` | Licensed professionals managing visits and availability |
| **ADMIN** | `/apps/admin` | Platform operators handling disputes and verification |
