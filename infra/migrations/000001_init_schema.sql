CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT CHECK (role IN ('CLIENT', 'PRO', 'ADMIN')),
    stripe_customer_id TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE professionals (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    stripe_account_id TEXT,
    last_known_location GEOGRAPHY(Point, 4326),
    service_radius INTEGER DEFAULT 5000,
    rating DECIMAL(3,2) DEFAULT 0.0
);

CREATE INDEX idx_pro_location ON professionals USING GIST (last_known_location);


CREATE TABLE profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    profile_type TEXT CHECK (profile_type IN ('MANAGED', 'INDEPENDENT')),
    relationship TEXT, -- e.g., 'Father', 'Self'
    name TEXT NOT NULL,
    dob DATE,
    blood_type TEXT,
    allergies JSONB, -- Flexible storage for medical lists
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_name TEXT NOT NULL,
    base_price_cents INTEGER NOT NULL, -- Storing in cents avoids rounding errors
    estimated_duration_minutes INTEGER NOT NULL,
    requires_material_check BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE bookings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id),
    pro_id UUID REFERENCES professionals(user_id), -- Nullable for broadcasting
    service_id UUID NOT NULL REFERENCES catalog(id),
    scheduled_time TIMESTAMP WITH TIME ZONE NOT NULL,
    status TEXT DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'CONFIRMED', 'COMPLETED_PRO', 'COMPLETED', 'DISPUTED', 'CANCELLED')),
    payment_status TEXT DEFAULT 'AUTHORIZED' CHECK (payment_status IN ('AUTHORIZED', 'CAPTURED', 'REFUNDED')),
    materials_snapshot JSONB, -- Legal proof of what was required at booking time
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

