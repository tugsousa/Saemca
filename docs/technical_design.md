# Technical Design Document: SaúdeEmCasa (MVP)

**Version:** 1.0
**Status:** Approved for Development
**Date:** January 2026

---

## 1. System Architecture Overview

The platform operates on a **Tri-App Architecture** (Client, Professional, Admin) connected to a central **Modular Monolith Backend**. The system uses Role-Based Access Control (RBAC) to serve different interfaces from a single domain.

### Core Stack
* **Frontend:** React (Web-based MVP) using TailwindCSS.
* **Backend:** Go (Golang) — Selected for high concurrency, type safety, and speed.
* **Database:** PostgreSQL with **PostGIS** extension (Required for geography-based proximity).
* **Payments:** **Stripe Connect** (Supports MB-WAY and automated Professional payouts).
* **Infrastructure:** Docker containers on Hetzner VPS + Cloudflare.
* **API Protocol:** REST (JSON) with strict Swagger/OpenAPI 3.0 definitions.


---

## 2. API & Communication Contract

To ensure stability between Frontend and Backend, all endpoints must adhere to a strict response wrapper.

### Standard Response Format
Every API response (Success or Error) must return this JSON structure:

```json
{
	"trace_id": "req-uuid-here",
	"success": true,
	"data": { "user_id": "123", "name": "Ana" },
	"meta": {
		"page": 1,
		"total": 50
	},
	"error": {
		"code": "MATERIAL_CHECK_FAILED",
		"message": "User has not confirmed material possession.",
		"trace_id": "req-12345"
	}
}
```

---

## 3. Data Layer (Schema Definitions)

### Users Table (Identity)
* `id` (PK, UUID)
* `email` (Unique, Indexed)
* `password_hash` (Argon2)
* `role` (Enum: CLIENT, PRO, ADMIN)
* `stripe_customer_id`

### Profiles Table (The Patient)
* `id` (PK, UUID)
* `user_id` (FK) — The "Account Owner" (e.g., the Son).
* `profile_type` (Enum: MANAGED, INDEPENDENT) — Allows management for elderly/children.
* `relationship` (String) — e.g., "Father", "Self", "Daughter".
* `name`, `dob`, `blood_type`
* `allergies` (JSONB)
* `notes` (Text, Encrypted at rest)

### Professionals Table (The Supply)
* `user_id` (FK)
* `stripe_account_id` (String) — For Connect Payouts.
* `last_known_location` (**GEOGRAPHY(Point, 4326)**) — Native GPS coordinates.
* `service_radius` (Integer, meters)
* `rating` (Float)

### Bookings Table (The Core Transaction)
* `id` (PK, UUID)
* `profile_id` (FK)
* `pro_id` (FK, Nullable for Broadcast requests)
* `service_type` (Enum)
* `scheduled_time` (Timestamp)
* `materials_snapshot` (JSONB) — A copy of the required materials list at the *moment of booking* (Legal protection).
* **status** (Enum: PENDING, CONFIRMED, COMPLETED_PRO, COMPLETED, DISPUTED, CANCELLED) 
    * *Note: COMPLETED_PRO triggers the 6h handshake timer; DISPUTED freezes funds.*
* **payment_status** (Enum: AUTHORIZED, CAPTURED, REFUNDED)

### Audit_Logs Table (GDPR Compliance)
* `id` (PK)
* `actor_id` (FK, User who performed the action)
* `action` (String, e.g., VIEW_HEALTH_DATA, ABORT_VISIT)
* `target_id` (UUID, The specific Booking or Profile accessed)
* `timestamp`
* `ip_address`


### Catalog Table
* `id` (PK, UUID)
* `service_name` (e.g., "General Nursing")
* `base_price` (Integer, cents)
* `estimated_duration` (Integer, minutes) -- Crucial for matching engine
* `surge_multiplier` (Decimal) -- For night/weekend rates

### Disputes Table
* `id` (PK, UUID)
* `booking_id` (FK)
* `opened_by` (FK, User) — CLIENT or PRO
* `opened_by_role` (Enum: CLIENT, PRO)
* `reason_category` (Enum: NO_SHOW, INCOMPLETE_SERVICE, SAFETY_CONCERN, ...)
* `description` (Text)
* `status` (Enum: OPEN, UNDER_REVIEW, RESOLVED, APPEALED)
* `resolution` (Enum: PRO_WIN, CLIENT_WIN, SPLIT, MUTUAL — nullable until resolved)
* `split_pro_percent` (Integer 0–100 — only for SPLIT and MUTUAL)
* `decision_message` (Text — shown to both parties)
* `admin_notes` (Text — internal only)
* `sla_paused` (Boolean — true while awaiting more info)
* `evidence_deadline`, `resolution_deadline` (Timestamps)
* `appeal_status` (Enum: null, REQUESTED, UPHELD, OVERTURNED)
* See [docs/disputes.md](disputes.md) for the full spec.

### Dispute_Evidence Table
* `id` (PK, UUID)
* `dispute_id` (FK)
* `submitted_by` (FK, User)
* `statement` (Text)
* `storage_key` (Text — optional file attachment in R2)

### Dispute_Messages Table
* `id` (PK, UUID)
* `dispute_id` (FK)
* `sender_id`, `recipient_id` (FK, User)
* `message` (Text)
* `is_internal` (Boolean — TRUE = admin-only, not shown to parties)

---

## 4. Component Definitions

### A. Frontend Components (The Interface)

**1. Multi-Profile Manager (Client App)**
* **Purpose:** Separates the "Payer" from the "Patient."
* **Logic:** A context-switcher in the header. All medical history/prescriptions fetch requests use the `Profile-ID` header, not just the `User-ID`.

**2. The "Material Gatekeeper" (Client App)**
* **Purpose:** Enforces the "Service-Only" model (Liability Shield).
* **Mechanism:**
    1.  User clicks "Book".
    2.  Modal opens fetching list from `/catalog/materials/{service_id}`.
    3.  User checks boxes.
    4.  Frontend receives a signed `verification_token` from backend upon checklist completion.
    5.  This token is required to hit the `POST /bookings` endpoint.

**3. "My Team" Priority Display (Client App)**
* **Purpose:** Increases retention (The "Loyal Loop").
* **Logic:** A horizontal scroll of past professionals. If a "Team" member is available, they appear first. If busy, they are grayed out with a "Next available: Tomorrow" label.

**4. Active Visit & Double-Handshake (Shared)**
* **Purpose:** Safety and Payment release.
* **Logic:**
    * **Pro Side:** Sees "Mark as Done".
    * **Client Side:** Receives push notification "Nurse finished. Confirm?".
    * **Chat:** Remains **OPEN** until both sides confirm (or 6-hour auto-timer expires).

### B. Backend Services (The Logic)

**1. Matching Engine (Time-Block Aware)**
* **Logic:** Instead of just checking a "start time," the engine checks for overlaps.
* **Query:** Find Pros where `requested_start` + `service.duration` does not overlap with existing `bookings` for that `pro_id`.

**2. Geo-Fenced Broadcasting (ASAP Requests)**
* **Purpose:** Handling immediate "Emergency" or "Right Now" requests.
* **Logic:**
    * Do not assign a specific Pro.
    * Query: Find all Pros where `ST_DWithin(pro.location, patient.location, 5000)` (5km radius) AND `status` is ONLINE.
    * Action: Send Push Notification to all matching Pros.
    * First to "Accept" gets the lock.

**3. Material Token Verifier**
* **Purpose:** Security against API bypass.
* **Logic:** The `POST /bookings` endpoint validates the `verification_token` (JWT). It ensures the user *actually* retrieved the checklist and confirmed it within the last 5 minutes.

**4. Cancellation Guard (Cron Job)**
* **Purpose:** Automates the hotel-style policy.
* **Logic:** Worker runs hourly.
    * Checks bookings starting in < 24h.
    * Updates metadata `cancellation_fee_active = true`.
    * If a user cancels, the Refund Service checks this flag to calculate the split (50/50).

**5. Profile Ownership & GDPR Middleware**
* **Purpose:** Ensures the User-ID in the JWT has permission to access the requested Profile-ID.
* **Logic:** * If `profile.user_id == jwt.user_id`, access granted.
    * If a profile is "Promoted" to INDEPENDENT, the link to the original creator is severed to maintain patient privacy.

**6. Time-Block Matching Engine**
* **Logic:** Bookings are treated as time blocks (Start Time + Estimated Duration).
* **Conflict Prevention:** Uses SQL `OVERLAPS` operator to ensure a Pro isn't booked for a new 45-min injection if they are already in a 1-hour wound care session.

**7. Double-Handshake & Dispute State Machine**
* **Purpose:** To ensure fair payment and professional accountability.
* **Full spec:** See [docs/disputes.md](disputes.md) — this is the authoritative document for all dispute logic.
* **Summary:**
    * When a Pro marks a visit as finished, status becomes `COMPLETED_PRO`.
    * Client has 6 hours to confirm or open a dispute. Auto-captures on timer expiry.
    * **Either CLIENT or PRO** can open a dispute → status `DISPUTED`, payment frozen.
    * 48h evidence window → Admin review (5 business day SLA) → PRO_WIN / CLIENT_WIN / SPLIT / MUTUAL.
    * Mutual resolution requires **explicit confirmation from both parties**.
    * 48h appeal window after resolution — requires new evidence, reviewed by a different Admin.
    * Professionals or clients who **lose 3 disputes** are flagged for manual review.

---

## 5. Directory Structure (Go Modular Monolith)

This structure follows Domain-Driven Design (DDD).

```text
/saude-em-casa
│
├── /backend
│   ├── /cmd
│   │   ├── /api                # Main REST API Server
│   │   └── /worker             # Cron Jobs (Cancellation Guard, Payouts)
│   │
│   ├── /internal               # Private Application Logic
│   │   ├── /auth               # JWT, Password Hashing
│   │   ├── /user               # User & Profile Management
│   │   │
│   │   ├── /booking            # The Core Engine
│   │   │   ├── handler.go      # HTTP Endpoints
│   │   │   ├── service.go      # Business Logic
│   │   │   ├── mutex_manager.go # Race Condition Locking
│   │   │   └── store.go        # DB Queries
│   │   │
│   │   ├── /geo                # Location Services
│   │   │   ├── location.go     # Wraps PostGIS logic
│   │   │   └── region.go       # Maps coords to "Parishes"
│   │   │
│   │   ├── /payment            # Escrow & Invoices
│   │   ├── /chat               # WebSockets
│   │   │
│   │   ├── /catalog            # Service Types & Pricing
│   │   │   ├── material_hasher.go # Generates security tokens
│   │   │   └── pricing_rules.go   # Validates Min/Max prices
│   │   │
│   │   └── /common             # Shared Utilities
│   │       ├── errors          # Standard Error Codes
│   │       └── response.go     # JSON Wrapper
│   │
│   ├── /platform               # Infrastructure (DB connection, S3)
│   ├── /migrations             # SQL Scripts (PostGIS setup, Tables)
│   └── go.mod
│
├── /frontend
│   ├── /src
│   │   ├── /apps               # The 3 Portals
│   │   │   ├── /client         # Ana's View
│   │   │   ├── /pro            # Clara's View
│   │   │   └── /admin          # God View
│   │   ├── /shared             # Components & Hooks
│   │   └── App.tsx             # Router
│   └── package.json
│
├── /docs                       # Specifications
└── /infra                      # Docker, Nginx, Makefiles
```

---

## 6. Functional Requirements (User Stories)

### Epic 1: Profile Management
* **US-1.1:** As a Family Manager, I want to create a separate profile for my father, so his medical history is distinct from mine.
* **US-1.2 (Security):** As a User, I want to switch profiles via a dropdown, ensuring I am viewing the correct "Patient Context."

### Epic 2: Booking & Matching
* **US-2.1:** As a Client, I want to see "My Team" (past nurses) first in search results to rebook trusted providers.
* **US-2.2 (Safety):** As a Client, I must pass a "Material Checklist" modal before payment, acknowledging I own the necessary medical supplies.
* **US-2.3 (Performance):** As a Client, I want to see results sorted by distance and rating instantly (using PostGIS).

### Epic 3: Financial & Cancellation
* **US-3.1:** As a Client, I want a clear warning if I am cancelling within the "Penalty Window" (<24h).
* **US-3.2:** As a Pro, I want to receive 50% of the fee for late cancellations to cover my reserved time.
* **US-3.3:** As an Admin, I want a "Force Cancel & Refund" button to resolve disputes instantly.

### Epic 4: Execution & Safety
* **US-4.1:** As a Pro, I want an "SOS Button" that alerts Admin and logs my location immediately.
* **US-4.2:** As a System, I want to prevent chat messages after the booking is "Double Confirmed" to finalize the transaction.

---

## 7. Infrastructure & Security

### Data Privacy (GDPR)
* **Encryption at Rest:** The PostgreSQL volume on Hetzner will use LUKS encryption.
* **Data Minimization:** Health notes are stored in `Profiles`, not `Users`.
* **Right to be Forgotten:** A dedicated `data_purger.go` worker hard-deletes medical data upon account closure, while retaining transaction IDs for 10 years (Fiscal Law).

### Deployment
* **Containerization:** Docker for consistency.
* **Reverse Proxy:** Nginx (or Traefik) handling SSL termination.
* **CDN:** Cloudflare for static asset caching and DDoS protection.

## 8. Failure Modes & Resiliency

| Scenario | System Response |
| :--- | :--- |
| **Race Condition on ASAP** | Database uses Optimistic Locking; second Pro to accept receives a `409 Conflict`. |
| **MB-WAY / Stripe Timeout** | Webhook endpoint is idempotent; retry logic ensures status is updated only once. |
| **Insecure Profile Access** | Middleware validates ownership/guardianship of Profile-ID on every request (IDOR Protection). |
| **PostGIS Precision** | Geography type ensures meter-accuracy for the 5km broadcast radius across Portugal. |

---

## 9. Finalized Stack Decisions

> These decisions were made during pre-development planning (March 2026) and supersede any ambiguity in earlier sections. Full rationale in [stack.md](stack.md).

### Authentication Pattern
- Access Token: JWT, 15-minute expiry, returned in response body, stored in JS memory only
- Refresh Token: Opaque UUID, hashed before storage in `refresh_tokens` table, sent via `httpOnly` cookie (30-day expiry)
- Never localStorage for JWTs — XSS vulnerability on a health data platform is unacceptable

### File Storage
- **Production:** Cloudflare R2 (zero egress fees, S3-compatible, EU-region available, provider-agnostic)
- **Development:** MinIO running in Docker (identical S3 API — same Go code, different env vars)
- Go library: `aws-sdk-go-v2/service/s3`

### Notifications
- **Email:** Resend (booking confirmations, receipts, verification)
- **SMS:** Vonage (time-sensitive alerts: "nurse en route", booking confirmed)
- **Browser Push:** Deferred — add after launch as enhancement

### Frontend State Management
- **Server state (API data):** TanStack Query
- **Client/UI state:** Zustand
- **Routing:** TanStack Router (type-safe)
- **Forms:** React Hook Form + Zod
- **Components:** shadcn/ui (Radix UI + Tailwind, owned code not runtime dependency)

### Go Module Name
`salusdomi.com/api`

### Missing Tables (additions to Section 3)
The following tables are required but were not in the initial schema:
- `refresh_tokens` — auth session management (token_hash, user_id, expires_at)
- `materials` — checklist items per catalog service (feeds the Material Gatekeeper)
- `reviews` — post-booking ratings (1-5 stars, links booking → profile → professional)
- `documents` — professional onboarding uploads (Cédula, insurance; stored in R2)
- `audit_logs` — GDPR access log (every read/write of health data)
- `fee_tiers` — platform fee rules (e.g., Founding 50 = 0%, standard = X%)

### Observability
- **Structured Logging:** `slog` stdlib, JSON output → stdout → Better Stack (Logtail)
- **Error Tracking:** Sentry (Go backend SDK + React frontend SDK)
- **Metrics/Uptime:** Grafana Cloud free tier

### GPS / Real-Time Location
- Browser GPS (`navigator.geolocation`) used for basic Pro location on web
- Real-time ASAP broadcasting deferred until native mobile app is built
- Web app is the primary platform; mobile app planned for a future phase