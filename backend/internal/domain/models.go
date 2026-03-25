package domain

import "time"

// User is the identity record. One per account.
type User struct {
	ID               string
	Email            string
	PasswordHash     string
	Role             string // CLIENT | PRO | ADMIN
	StripeCustomerID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Profile is the patient record. A user can have multiple profiles
// (e.g. a son managing profiles for himself and his elderly father).
type Profile struct {
	ID           string
	UserID       string
	ProfileType  string // MANAGED | INDEPENDENT
	Relationship string // e.g. "Father", "Self"
	Name         string
	Dob          time.Time
	BloodType    string
	Allergies    []byte // JSONB
	Notes        string // Encrypted at rest
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Professional extends User with supply-side data.
type Professional struct {
	UserID            string
	StripeAccountID   string // Fixed typo from original stub
	LastKnownLocation string // PostGIS GEOGRAPHY — parsed separately
	ServiceRadius     int    // meters
	Rating            float64
	IsOnline          bool
	FeetierID         string
	Verified          bool
	VerifiedAt        *time.Time
}

// Catalog is a service type offered on the platform (e.g. "General Nursing").
type Catalog struct {
	ID                      string
	ServiceName             string
	BasePriceCents          int // Stored in cents to avoid float rounding
	EstimatedDurationMinutes int
	RequiresMaterialCheck   bool
	SurgeMultiplier         float64
	CreatedAt               time.Time
}

// Booking is the core transaction record.
type Booking struct {
	ID                string
	ProfileID         string
	ProID             string    // Nullable — empty for ASAP broadcast bookings
	ServiceID         string
	ScheduledTime     time.Time
	EndTime           *time.Time
	Status            string // PENDING | CONFIRMED | COMPLETED_PRO | COMPLETED | DISPUTED | CANCELLED
	PaymentStatus     string // AUTHORIZED | CAPTURED | REFUNDED
	MaterialsSnapshot []byte // JSONB — immutable legal record of required materials at booking time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
