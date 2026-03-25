# Dispute Resolution — SALUSDOMI

**Version:** 1.0
**Status:** Approved for Development
**Last Updated:** March 2026

This document is the single source of truth for the dispute resolution flow.
All implementation (backend services, frontend UI, admin backoffice) must follow this spec.

---

## Overview

A dispute is triggered when either party is unsatisfied with the outcome of a completed visit.
While a dispute is open, payment is frozen (remains in `AUTHORIZED` state) — no money moves
until an Admin resolves or mediates the case.

---

## Who Can Open a Dispute

Both **CLIENT** and **PRO** can open a dispute, but for different reasons.

### Client reasons
| Code | Label |
|---|---|
| `NO_SHOW` | Professional did not show up |
| `INCOMPLETE_SERVICE` | Service was not fully completed |
| `SAFETY_CONCERN` | I felt unsafe during the visit |
| `UNAUTHORIZED_ACTION` | Something was done without my consent |
| `OTHER` | Other (requires description) |

### Professional reasons
| Code | Label |
|---|---|
| `CLIENT_ABUSIVE` | Client was abusive or threatening |
| `FALSE_DISPUTE` | Client opened a false dispute against me |
| `PAYMENT_ISSUE` | I believe I should be paid for work completed |
| `OTHER` | Other (requires description) |

---

## Full State Machine

```
COMPLETED_PRO  (Pro marks visit as done)
      │
      ├── Client confirms ──────────────────────────► COMPLETED
      │                                               (payment captured)
      ├── 6-hour timer expires (no response) ───────► COMPLETED
      │                                               (auto-capture)
      └── Either party opens dispute ───────────────► DISPUTED
                                                           │
                                                   ┌───────┴────────┐
                                               48h evidence window
                                               Chat → READ-ONLY
                                               Dispute Thread opens
                                               (Admin ↔ each party, private)
                                                           │
                                                   Admin Review
                                                   (5 business day SLA)
                                                   Admin can pause SLA
                                                   by requesting more info
                                                           │
                                          ┌────────────────┼───────────────┐
                                          ▼                ▼               ▼
                                      PRO_WIN         CLIENT_WIN         SPLIT
                                   (full capture)   (full refund)  (admin % split)
                                          │                │               │
                                          └────────────────┼───────────────┘
                                                           │
                                                    ┌──────┴──────┐
                                                    │             │
                                                 MUTUAL        Decision
                                               RESOLUTION      sent to
                                           (both parties       both parties
                                            must explicitly        │
                                            confirm via         48h Appeal
                                            in-app button)      Window
                                                    │               │
                                               COMPLETED    ┌───────┴────────┐
                                                         Appeal          No appeal
                                                         requested       └──► FINAL
                                                             │
                                                     Different admin
                                                     reviews (appeals queue)
                                                             │
                                                   UPHELD or OVERTURNED
                                                             │
                                                           FINAL
                                                    (no further escalation)
```

---

## Phase 1 — Opening a Dispute

**Trigger:** Either party clicks "Open Dispute" on an active booking in `COMPLETED_PRO` status.

**What happens immediately:**
1. Booking status → `DISPUTED`
2. Payment remains frozen (`AUTHORIZED` — no capture, no release)
3. Existing booking chat → **READ-ONLY** (preserved as evidence, no new messages)
4. A private **Dispute Thread** is created per party:
   - Admin ↔ Client (Client cannot see Admin ↔ Pro thread)
   - Admin ↔ Pro (Pro cannot see Admin ↔ Client thread)
5. `evidence_deadline` set to `NOW() + 48 hours`
6. `resolution_deadline` set to `NOW() + 5 business days`
7. Both parties notified (email + SMS): dispute opened, 48h to submit evidence
8. Admin notified: new dispute in queue

**Opening party must provide:**
- Reason category (from the list above)
- Written description (min 20 characters)
- Optional file attachment (photo, document — stored in R2)

---

## Phase 2 — Evidence Window (48 hours)

Both parties can submit evidence via the Dispute Thread:
- Written statement
- File attachments (photos, documents)

The **non-opening party** is also notified and invited to submit their side.

**Rules:**
- Each party can submit multiple pieces of evidence within the 48h window
- After 48h, evidence submission is locked — no new evidence from either party
- Exception: Admin can request additional info (see Phase 3)
- The original booking chat transcript is automatically included as evidence

**After the 48h window closes:**
Dispute status → `UNDER_REVIEW`. Admin receives notification to begin review.

---

## Phase 3 — Admin Review

Admin reviews the dispute in the backoffice.

### What Admin sees

**Booking panel:**
- Service type, scheduled time, duration, address
- Total amount frozen, platform fee, net to Pro if released

**Client panel:**
- Identity, profile, booking history
- Their written statement + attachments
- Their overall dispute history: disputes opened, disputes lost, flag status

**Professional panel:**
- Identity, verification status, rating, booking history
- Their written statement + attachments
- Their dispute rate: disputes opened against them, disputes lost, flag status
- Documents on file (Cédula, insurance)

**Evidence panel:**
- Full booking chat transcript (read-only)
- All submitted evidence from both parties
- Audit log of all system actions on this booking

**SLA panel:**
- Time remaining until resolution deadline
- Whether SLA is paused (pending more info request)

### Admin actions

| Action | Effect |
|---|---|
| **Release to Pro** | Full payment capture → booking `COMPLETED` |
| **Refund Client** | Release authorization → booking `CANCELLED` |
| **Split** | Admin enters Pro % (0–100). Stripe partial capture + partial refund |
| **Propose Mutual Resolution** | Admin proposes terms; both parties must confirm |
| **Request more info** | Sends message in Dispute Thread to one or both parties; SLA timer paused until response |
| **Flag user for review** | Adds `manual_review` flag to user record (see Flagging section) |

**Admin must always write:**
- A **decision message** (visible to both parties — explains the outcome)
- An **internal note** (admin-only — reasoning, context, anything not for parties)

---

## Phase 4 — Mutual Resolution

If Admin selects "Propose Mutual Resolution":

1. Admin writes the proposed terms (e.g. "50% refund to client, 50% captured for Pro")
2. Both parties receive the proposal in their Dispute Thread
3. Each party sees an **"Accept"** and **"Decline"** button
4. **Both must explicitly click "Accept"** for the mutual resolution to proceed
5. If either party declines → dispute returns to Admin with declined status
6. If no response within 24h → treated as declined, Admin resolves unilaterally
7. Once both accept → payment processed per agreed terms → booking `COMPLETED`

**Why explicit confirmation?**
Legal protection. A mutual resolution with both parties' explicit acceptance is much
stronger than a unilateral admin decision if the case ever goes further.

---

## Phase 5 — Resolution & Notification

Once Admin resolves (any method):

1. Booking status updated: `COMPLETED` (Pro wins or mutual) or `CANCELLED` (Client wins)
2. Payment processed immediately
3. Both parties receive the decision message via email + SMS
4. 48-hour appeal window opens

---

## Phase 6 — Appeals

**Rules:**
- Either party may appeal within **48 hours** of resolution notification
- Appeals require **new evidence** not previously submitted — no new evidence = appeal rejected automatically
- **One appeal per dispute maximum** — the appeal decision is final
- Appeal is reviewed by a **different Admin** than the one who made the original decision (prevents anchoring bias)

**Appeal process:**
1. Party clicks "Appeal Decision" (visible for 48h after resolution)
2. They must attach or write new evidence — submitting without new evidence shows an error
3. Appeal goes to a separate **Appeals Queue**
4. Original Admin is notified their decision is under appeal (for context, they cannot act on it)
5. Reviewing Admin can: **Uphold** (original decision stands) or **Overturn** (new outcome)
6. Final decision sent to both parties — no further escalation

---

## Manual Review Flagging

**Trigger:** A user (CLIENT or PRO) loses **3 disputes** (configurable — update `DISPUTE_FLAG_THRESHOLD` in config).

> "Lost" means the dispute was resolved against them.
> Disputes they opened and won do not count.
> Disputes they opened and lost do count.

**What happens when flagged:**
1. `manual_review_flag` set to `true` on the user record
2. Admin receives notification: "User X has been flagged for manual review"
3. User appears in the **Flagged Users** queue in the backoffice
4. Admin reviews and can:
   - **Clear the flag** (false positives — legitimate losing streak)
   - **Issue a warning** (notification sent to user)
   - **Suspend the account** (manual action — no auto-suspension yet)

**Future:** Once there is enough data to tune the threshold, auto-suspension can be enabled.

---

## Database Schema

### `disputes` table
```sql
disputes
├── id                   UUID PK
├── booking_id           UUID FK bookings(id)
├── opened_by            UUID FK users(id)
├── opened_by_role       TEXT  -- CLIENT | PRO
├── reason_category      TEXT  -- NO_SHOW | INCOMPLETE_SERVICE | ...
├── description          TEXT
├── status               TEXT  -- OPEN | UNDER_REVIEW | RESOLVED | APPEALED
├── resolution           TEXT  -- PRO_WIN | CLIENT_WIN | SPLIT | MUTUAL | null
├── split_pro_percent    INTEGER  -- 0–100, only for SPLIT and MUTUAL
├── decision_message     TEXT  -- Shown to both parties
├── admin_notes          TEXT  -- Internal only
├── resolved_by          UUID FK users(id)  -- Admin
├── resolved_at          TIMESTAMP
├── evidence_deadline    TIMESTAMP  -- opened_at + 48h
├── resolution_deadline  TIMESTAMP  -- opened_at + 5 business days
├── sla_paused           BOOLEAN DEFAULT FALSE
├── appeal_status        TEXT  -- null | REQUESTED | UPHELD | OVERTURNED
├── appeal_opened_at     TIMESTAMP
├── appeal_resolved_by   UUID FK users(id)  -- Different admin
├── appeal_resolved_at   TIMESTAMP
├── created_at           TIMESTAMP
└── updated_at           TIMESTAMP
```

### `dispute_evidence` table
```sql
dispute_evidence
├── id            UUID PK
├── dispute_id    UUID FK disputes(id)
├── submitted_by  UUID FK users(id)
├── statement     TEXT
├── storage_key   TEXT  -- Optional file in R2/MinIO
├── mime_type     TEXT
└── created_at    TIMESTAMP
```

### `dispute_messages` table
```sql
dispute_messages
├── id            UUID PK
├── dispute_id    UUID FK disputes(id)
├── sender_id     UUID FK users(id)
├── recipient_id  UUID FK users(id)
├── message       TEXT
├── is_internal   BOOLEAN  -- TRUE = admin-only note, not shown to parties
└── created_at    TIMESTAMP
```

### Changes to `users` table (migration)
```sql
ALTER TABLE users
  ADD COLUMN disputes_lost        INTEGER DEFAULT 0,
  ADD COLUMN manual_review_flag   BOOLEAN DEFAULT FALSE,
  ADD COLUMN manual_review_reason TEXT;
```

---

## Configuration Values

These live in `Config` (env vars) so they can be changed without code deploys:

| Key | Default | Description |
|---|---|---|
| `DISPUTE_EVIDENCE_WINDOW_HOURS` | 48 | Hours each party has to submit evidence |
| `DISPUTE_SLA_BUSINESS_DAYS` | 5 | Business days for admin to resolve |
| `DISPUTE_APPEAL_WINDOW_HOURS` | 48 | Hours after resolution to file appeal |
| `DISPUTE_MUTUAL_ACCEPT_HOURS` | 24 | Hours parties have to accept/decline mutual proposal |
| `DISPUTE_FLAG_THRESHOLD` | 3 | Lost disputes before manual review flag triggers |

---

## What the Admin Backoffice Must Have

### Dispute Queue (list view)
- All open disputes sorted by SLA urgency (soonest deadline first)
- Filter by: status, opened_by_role, SLA breached (yes/no)
- At-a-glance: booking ID, parties involved, reason, days open, SLA status (green/yellow/red)

### Appeals Queue (separate list)
- All disputes with `appeal_status = REQUESTED`
- Assigned to a different admin than the original resolver

### Flagged Users Queue
- All users with `manual_review_flag = TRUE`
- Shows: user ID, role, disputes lost, last flag date, current account status

### Dispute Detail View
- Full panels as described in Phase 3 above
- Action buttons with confirmation dialogs (prevent accidental resolution)
- All actions logged to `audit_logs`
