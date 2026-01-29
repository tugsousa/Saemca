# User Stories: SaúdeEmCasa (MVP)

### Epic 1: Profile Management & Managed Care
* **US-1.1:** As a Family Manager, I want to create a separate profile for a dependent (e.g., elderly father) without requiring them to have an email, so I can manage their care centrally.
* **US-1.2:** As a User, I want to switch profiles via a dropdown, ensuring the system validates my ownership of that profile context via a secure `Profile-ID` header.
* **US-1.3:** As a Family Manager, I want to define a profile as "MANAGED" or "INDEPENDENT" to ensure GDPR-compliant data handling.

### Epic 2: Booking & High-Performance Matching
* **US-2.1:** As a Client, I want to see "My Team" (past nurses) prioritized in search results to rebook trusted providers.
* **US-2.2:** As a Client, I must pass a "Material Checklist" and receive a signed verification token before payment, acknowledging I possess the necessary medical supplies.
* **US-2.3:** As a Client, I want to see results sorted by distance (calculated via PostGIS Geography) and rating instantly within a 5km radius.

### Epic 3: Financial & Dispute Resolution
* **US-3.1:** As a Client, I want a clear warning if I am cancelling within the <24h window, acknowledging the automated 50% split penalty.
* **US-3.2:** As a Pro, I want my earnings (minus platform fees) to be automatically routed to my verified bank account via Stripe Connect.
* **US-3.3:** As an Admin, I want to manually resolve "DISPUTED" bookings to either capture authorized funds for the Pro or release them back to the Client.

### Epic 4: Execution & Handshake Safety
* **US-4.1:** As a Pro, I want an "SOS Button" that logs my current GPS coordinates and alerts Admin immediately.
* **US-4.2:** As a System, I want to move a booking to `COMPLETED_PRO` once the nurse finishes, starting a 6-hour "Handshake Timer" for the client to confirm or dispute.
* **US-4.3:** As a System, I want to disable chat functionality once a booking is finalized to prevent off-platform transactions.

### Epic 5: Professional Onboarding
* **US-5.1:** As a Professional, I want to upload my Cédula and Insurance via a secure document portal for manual verification.
* **US-5.2:** As an Admin, I want to view a "Verification Queue" of pending documents so I can manually approve the "Founding 50" nurses.
* **US-5.3:** As a "Founding 50" Pro, I want my platform fee set to 0% to reward early adoption.

### Epic 6: Reputation & Gamification
* **US-6.1:** As a Client, I want to rate my nurse (1-5 Stars), triggering an immediate recalculation of their public rating.
* **US-6.2:** As a Professional, I want to see a "Gold Tier Progress Bar" based on successful completions and low dispute rates.

### Epic 7: Privacy-First Logistics
* **US-7.1:** As a Client, I want to receive SMS/Push updates when a nurse is "En Route" without needing constant map-tracking, preserving the Pro's privacy.
* **US-7.2:** As a System, I want to use PostGIS `ST_DWithin` to automatically detect when a Pro arrives at a patient's address to enable the "Start Visit" button.