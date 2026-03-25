-- One review per booking (UNIQUE constraint on booking_id).
-- A review can only be created once a booking reaches COMPLETED status.
-- The pro's public rating is recalculated from this table on each new review.

CREATE TABLE reviews (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id UUID NOT NULL UNIQUE REFERENCES bookings(id),
    profile_id UUID NOT NULL REFERENCES profiles(id),          -- Who wrote the review
    pro_id     UUID NOT NULL REFERENCES professionals(user_id), -- Who is being reviewed
    rating     INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment    TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_reviews_pro_id ON reviews(pro_id);
