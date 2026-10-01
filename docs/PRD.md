# Hotel Bookings API — Product Requirements

## Purpose

Provide a small, inspectable REST API for a hotel-booking coding exercise. It lets an API consumer find a hotel, discover rooms suitable for a party and date range, create a booking, and retrieve it by reference. The goal is to demonstrate clear Go design and correctness around the central booking invariant—not to simulate a full hotel-management product.

## In scope

- Search hotels by case-insensitive name substring.
- Find individual rooms available for a given hotel, stay, and party size.
- Create a booking and return a unique public reference.
- Retrieve booking details by that reference.
- Provide deterministic seed/reset endpoints for evaluators.
- Log a simulated confirmation email asynchronously two seconds after a successful booking.
- Supply an OpenAPI contract, executable HTTP requests, automated tests, and Docker/Compose local execution.

## Product decisions and rationale

| Decision | Choice | Rationale |
| --- | --- | --- |
| Inventory | One seeded hotel, “The Grand Hotel” (ID 1): 2 single (capacity 1), 2 double (2), 2 deluxe (4) | The brief requires six rooms of three types but not their mix. An even split exercises every capacity rule. |
| Stay dates | ISO-8601 calendar dates; check-in inclusive, checkout exclusive | A new guest can arrive on the day the previous guest leaves. |
| Room assignment | The service assigns the smallest room that fits the party, then the lowest room number | Guests ask for a stay, not a specific room. Filling the smallest adequate room first keeps large rooms free for large parties. |
| Availability | A list of individual qualifying rooms; no availability is `200 []` | Showing concrete rooms makes the assignment rule visible and checkable. An empty result is a valid answer, not an error. |
| Booking details | Party size, lead guest name, and email | Party size drives room assignment; the name and email identify who to confirm the booking with. |
| Booking reference | Opaque `HBK-` reference, unique in the database | Guests can share a reference safely because it reveals nothing about database IDs or booking volume. |
| Maximum stay | 30 nights | Caps the number of night rows one booking creates and rejects obviously mistaken date ranges. |
| Test data | Idempotent seed and destructive reset, enabled by configuration | Evaluators can repeat the workflow from a known state. Deployments can switch the destructive endpoints off. |

## Business invariants

1. A room is never assigned to two bookings for the same night.
2. One booking assigns exactly one room for its entire stay; guests never change rooms.
3. Party size never exceeds the assigned room's capacity.
4. A booking reference is unique.
5. Every room is a single, double, or deluxe room with a positive capacity.

The database enforces invariants 1, 4, and 5 with constraints; the booking service enforces 2 and 3.

## Implemented scope checklist

- [x] Hotel name search
- [x] Date- and party-size-aware individual room availability
- [x] One-room, non-overlapping booking creation with a unique reference
- [x] Booking lookup by reference
- [x] Seed/reset evaluator workflow
- [x] Asynchronous two-second confirmation simulation

## Explicit non-goals

Authentication, booking cancellation or modification, pricing, payments, hotel/room administration, pagination, and a rendered Swagger UI are intentionally excluded. The brief says authentication is unnecessary. The other exclusions keep the scope to one complete booking flow rather than several half-built features.
