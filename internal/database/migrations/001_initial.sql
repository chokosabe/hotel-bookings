CREATE TABLE hotels (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE rooms (
    id INTEGER PRIMARY KEY,
    hotel_id INTEGER NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
    number TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('single', 'double', 'deluxe')),
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    UNIQUE (hotel_id, number)
);

CREATE TABLE bookings (
    id INTEGER PRIMARY KEY,
    reference TEXT NOT NULL UNIQUE,
    room_id INTEGER NOT NULL REFERENCES rooms(id),
    check_in TEXT NOT NULL,
    check_out TEXT NOT NULL,
    guest_count INTEGER NOT NULL CHECK (guest_count > 0),
    lead_guest_name TEXT NOT NULL,
    lead_guest_email TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE booking_nights (
    booking_id INTEGER NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    room_id INTEGER NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    stay_date TEXT NOT NULL,
    PRIMARY KEY (booking_id, stay_date),
    UNIQUE (room_id, stay_date)
);
