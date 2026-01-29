type User struct {
	ID string
	Email string
	PasswordHash string
	Role string
	CreatedAt time.Time
}

type Profile struct {
	ID string
	UserId string
	ProfileType string
	Relationship string
	Name string
	Dob time.Time
	BloodType string
	Allergies []byte
	Notes string
	CreatedAt time.Time
}

type Professional struct {
	UserId string
	StripeAccoutId string
	LastKnownLocation string
	ServiceRadius uint
	Rating float32
}

type Catalog struct {
	ID string
	ServiceName string
	BasePriceCents uint
	EstimatedDurationMinutes uint
	RequiresMaterialCheck bool
	CreatedAt time.Time
}

type Booking struct {
	ID string
	ProfileId string
	ProId string
	ServiceId string
	ScheduledTime time.Time
	Status string
	PaymentStatus string
	MaterialsSnapshot string
	CreatedAt time.Time
}
