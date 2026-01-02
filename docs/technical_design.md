*This is the blueprint we worked on.*

```markdown
# Technical Design Document (TDD)
**Project:** SaúdeEmCasa MVP
**Version:** 1.0

## 1. System Overview
The platform uses a **Tri-App Architecture** served from a single Role-Based Frontend.
- **Client App:** For families booking care.
- **Pro App:** For nurses managing jobs/earnings.
- **Admin App:** For verification and dispute resolution.

All apps communicate with a monolithic **Go Backend** via REST/WebSockets.

## 2. Core Components (Backend)

### A. Identity & Auth (`/internal/auth`)
- **JWT Implementation:** Tokens contain `user_id` and `role`.
- **RBAC Middleware:** Enforces strict separation (e.g., Clients cannot access `/pro/earnings`).

### B. User & Profiles (`/internal/user`)
- **Relationship:** One `User` (Payer) has Many `Profiles` (Patients).
- **Safety:** Medical history (allergies) is linked strictly to the `Profile`, not the `User`.

### C. Booking Engine (`/internal/booking`)
- **Matching Algorithm:**
  1. Priority: "My Team" (Previous Providers).
  2. Filter: Availability & Service Capability.
  3. Sort: Distance & Rating.
- **State Machine:**
  `PENDING` -> `CONFIRMED` -> `IN_PROGRESS` -> `COMPLETED`.
  *Note: "Completed" state requires a double-handshake (Pro + Client confirmation).*

### D. Financials (`/internal/payment`)
- **Escrow Logic:** Funds are `AUTHORIZED` on booking and `CAPTURED` on completion.
- **Cancellation Guard:** A background worker runs hourly to flag bookings <24h away, enabling the 50% cancellation fee logic.

### E. Catalog (`/internal/catalog`)
- **Material Check:** Each Service ID maps to a specific JSON list of required materials.
- **Smart Corridor:** Price updates are validated against Admin-set Min/Max ranges.

## 3. Data Schema (Key Entities)

| Table | Key Fields | Purpose |
| :--- | :--- | :--- |
| **Users** | `id`, `email`, `role`, `stripe_id` | Authentication & Payments |
| **Profiles** | `id`, `user_id`, `medical_notes` | Clinical Context |
| **Bookings** | `id`, `pro_id`, `profile_id`, `status` | The Core Transaction |
| **Materials** | `service_id`, `item_list_json` | The Liability Shield |

## 4. Infrastructure
- **Containerization:** All services run in Docker.
- **Reverse Proxy:** Nginx handles routing between Frontend (`/`) and Backend (`/api`).