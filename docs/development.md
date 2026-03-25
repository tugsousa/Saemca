# Development Guide — SALUSDOMI

Everything a developer needs to set up, run, and contribute to this project.

---

## Prerequisites

| Tool | Version | Install |
|---|---|---|
| Go | 1.23+ | https://go.dev/dl/ |
| Node.js | 20+ (LTS) | https://nodejs.org |
| Docker Desktop | Latest | https://www.docker.com/products/docker-desktop |
| Make | Any | Pre-installed on Mac/Linux; Windows: via Chocolatey |

---

## First-Time Setup

### 1. Clone the repo
```bash
git clone https://github.com/salusdomi/salusdomi.git
cd salusdomi
```

### 2. Start infrastructure (DB + Storage)
```bash
cd infra
cp .env.example .env   # fill in values (see .env.example)
docker compose up -d
```

This starts:
- PostgreSQL 18 + PostGIS on `localhost:5432`
- MinIO (S3-compatible storage) on `localhost:9000` (console at `localhost:9001`)

### 3. Run database migrations
```bash
make migrate-up
```

### 4. Start the backend
```bash
cd backend
go mod download
go run ./cmd/api
```
API available at `http://localhost:8080`

### 5. Start the frontend
```bash
cd frontend
npm install
npm run dev
```
App available at `http://localhost:5173`

---

## Environment Variables

All secrets live in `/infra/.env` (never committed to git).

```env
# Database
DB_USER=salusdomi
DB_PASSWORD=changeme
DB_NAME=salusdomi_db
DB_HOST=localhost
DB_PORT=5432

# Auth
JWT_SECRET=your-256-bit-secret
JWT_EXPIRY_MINUTES=15
REFRESH_TOKEN_EXPIRY_DAYS=30

# Storage (MinIO for dev)
STORAGE_ENDPOINT=http://localhost:9000
STORAGE_BUCKET=salusdomi-uploads
STORAGE_ACCESS_KEY=minioadmin
STORAGE_SECRET_KEY=minioadmin

# Payments
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...

# Notifications
RESEND_API_KEY=re_...
VONAGE_API_KEY=...
VONAGE_API_SECRET=...

# Observability
SENTRY_DSN=https://...
```

---

## Project Structure

```
/SALUSDOMI
├── /backend
│   ├── /cmd
│   │   ├── /api        # Main API server entry point (main.go)
│   │   └── /worker     # Cron job entry point (cancellation guard, payouts)
│   ├── /internal       # All application logic (private — not importable externally)
│   │   ├── /auth       # Login, JWT, refresh tokens
│   │   ├── /user       # User and profile management
│   │   ├── /catalog    # Service types, pricing, materials
│   │   ├── /booking    # Core booking engine and state machine
│   │   ├── /payment    # Stripe Connect, escrow
│   │   ├── /geo        # PostGIS proximity queries
│   │   ├── /chat       # WebSocket chat
│   │   ├── /domain     # Shared domain models (structs)
│   │   └── /common     # Shared utilities (response wrapper, errors)
│   ├── /platform       # Infrastructure (DB connection pool, S3 client)
│   └── go.mod
├── /frontend
│   └── /src
│       ├── /apps
│       │   ├── /client     # Client portal (booking, profile management)
│       │   ├── /pro        # Professional portal (availability, visits)
│       │   └── /admin      # Admin portal (disputes, verification queue)
│       ├── /shared         # Shared components, hooks, utils
│       └── App.tsx
├── /infra
│   ├── /migrations         # SQL migration files (numbered sequentially)
│   ├── docker-compose.yml
│   ├── .env.example
│   └── nginx.conf
├── /e2e                    # Playwright end-to-end tests
├── /docs                   # Technical documentation
├── CLAUDE.md               # AI assistant context (read this first)
└── README.md
```

---

## Git Workflow

### Branches
- `main` — production-ready code only. Never commit directly.
- `dev` — integration branch. Features merge here first.
- `feature/your-feature-name` — individual feature branches off `dev`.
- `fix/bug-description` — hotfix branches.

### Commit Convention
Follow [Conventional Commits](https://www.conventionalcommits.org/):
```
feat(booking): add time-block overlap validation
fix(auth): refresh token not expiring on logout
docs(stack): add Vonage rationale
test(booking): add integration test for ASAP broadcast
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`

### Pull Request Rules
- Every PR must target `dev` (not `main`)
- Every PR must have passing tests (`make test-unit && make test-integration`)
- At least one reviewer when there are multiple team members
- Keep PRs focused — one concern per PR

---

## Backend Conventions

### Layer Rules (strict)
```
handler.go  →  Parse HTTP request, call service, return response
service.go  →  Business logic only, calls repository and other services
repository.go  →  SQL queries only, returns domain types
```
Never skip layers. A handler must not call a repository directly.

### Error Handling
Return errors up the call stack — don't swallow them. The handler layer decides the HTTP status code.

```go
// Service layer — return descriptive errors
func (s *BookingService) Create(ctx context.Context, req CreateBookingRequest) (*Booking, error) {
    available, err := s.repo.CheckAvailability(ctx, req.ProID, req.ScheduledTime)
    if err != nil {
        return nil, fmt.Errorf("checking pro availability: %w", err)
    }
    if !available {
        return nil, ErrProNotAvailable
    }
    // ...
}
```

Define domain-specific error variables in each package:
```go
var (
    ErrProNotAvailable   = errors.New("professional is not available at the requested time")
    ErrProfileNotOwned   = errors.New("user does not own this profile")
)
```

### Logging
Use `slog` with context, always include `trace_id`:
```go
slog.InfoContext(ctx, "booking created",
    slog.String("booking_id", booking.ID),
    slog.String("pro_id", booking.ProID),
    slog.String("trace_id", traceID),
)
```

---

## Frontend Conventions

### Component Rule
Components render UI only. Business logic lives in hooks.

```tsx
// Bad — logic in component
function BookingButton({ proID }) {
  const [loading, setLoading] = useState(false)
  async function handleBook() {
    setLoading(true)
    await fetch('/api/bookings', { method: 'POST', ... })
    setLoading(false)
  }
  return <button onClick={handleBook}>Book</button>
}

// Good — logic in hook
function BookingButton({ proID }) {
  const { mutate: createBooking, isPending } = useCreateBooking()
  return <button onClick={() => createBooking({ proID })} disabled={isPending}>Book</button>
}
```

### File Naming
- Components: `PascalCase.tsx` (e.g., `BookingCard.tsx`)
- Hooks: `camelCase.ts` with `use` prefix (e.g., `useCreateBooking.ts`)
- Utils: `camelCase.ts` (e.g., `formatCurrency.ts`)

---

## Database Conventions

### Migrations
Migrations live in `/infra/migrations/` and are numbered sequentially:
```
000001_init_schema.sql
000002_add_refresh_tokens.sql
000003_add_materials_table.sql
```
**Never modify an existing migration file.** If you need to change a table, create a new migration.

### Running Migrations
```bash
make migrate-up      # apply all pending migrations
make migrate-down    # roll back the last migration
make migrate-status  # show current migration state
```

---

## Testing

### Run Tests
```bash
make test-unit          # unit tests only (fast, no Docker required)
make test-integration   # integration tests (requires Docker)
make test-e2e           # end-to-end tests (requires full stack running)
make test-all           # runs all three in sequence
```

### Writing Tests

**Unit test example (service layer):**
```go
func TestBookingService_CannotDoubleBook(t *testing.T) {
    // Arrange
    svc := NewBookingService(mockRepo)

    // Act
    _, err := svc.Create(ctx, CreateBookingRequest{...})

    // Assert
    require.ErrorIs(t, err, ErrProNotAvailable)
}
```

**Integration test example (repository layer):**
```go
func TestBookingRepo_FindAvailable(t *testing.T) {
    db := testhelpers.NewTestDB(t)  // spins up real PostgreSQL via testcontainers
    repo := NewBookingRepository(db)

    bookings, err := repo.FindAvailablePros(ctx, location, time)
    require.NoError(t, err)
    assert.NotEmpty(t, bookings)
}
```

---

## Useful Make Commands

```bash
make dev              # start all infrastructure (Docker)
make migrate-up       # apply pending migrations
make migrate-down     # roll back last migration
make test-unit        # run unit tests
make test-integration # run integration tests
make test-e2e         # run Playwright e2e tests
make lint             # run golangci-lint
make build            # build Go binaries (api + worker)
make swagger          # regenerate OpenAPI docs from code annotations
```
