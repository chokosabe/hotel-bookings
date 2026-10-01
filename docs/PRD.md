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


| Decision          | Choice                                                                        | Why this is the best fit for this exercise                                                                                            |
| ----------------- | ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| Inventory         | One seeded hotel, “The Grand Hotel”; 2 single (1), 2 double (2), 2 deluxe (4) | The brief requires six rooms and three types but not their distribution. We balanced the room composition for demonstration purposes. |
| Stay dates        | ISO-8601 dates; check-in inclusive, checkout exclusive                        | A new guest can arrive on the prior guest’s checkout date.                                                                            |
| Room assignment   | The service assigns the lowest adequate capacity, then lowest room number     | Costumers dont need to know internal room IDs; larger parties get larger rooms.                                                       |
| Availability      | Individual qualifying rooms; empty availability is `200 []`                   | Result is easier to test.                                                                                                             |
| Booking details   | Party size, lead guest name, and email                                        | We need this info to autoallocate rooms by size and also notify the person booking.                                                   |
| Booking reference | Opaque `HBK-` reference, unique in the database                               | Public identifiers remain separate from database IDs and are safe to share with a guest.                                              |
| Maximum stay      | 30 nights                                                                     | Standard.                                                                                                                             |
| Test data         | Idempotent seed and destructive reset, enabled by configuration               | The required evaluator workflow is repeatable. Prod deployments can disable destructive endpoints.                                    |


## Business invariants

1. A hotel has rooms of only the single, double, and deluxe types in the supplied seed data.
2. The seeded hotel has exactly six rooms.
3. A room is never assigned to two bookings for the same night.
4. One booking assigns exactly one room for its entire stay; guests never change rooms.
5. A booking reference is unique.
6. Party size never exceeds the selected room capacity.

## Implemented scope checklist

- [x] Hotel name search
- [x] Date- and party-size-aware individual room availability
- [x] One-room, non-overlapping booking creation with a unique reference
- [x] Booking lookup by reference
- [x] Seed/reset evaluator workflow
- [x] Asynchronous two-second confirmation simulation

## Explicit non-goals

Authentication, booking cancellation or modification, pricing, payments, hotel/room administration, pagination, and a rendered Swagger UI are intentionally excluded. The challenge says authentication is unnecessary; the remaining exclusions preserve a complete, focused booking flow rather than incomplete product fragments.