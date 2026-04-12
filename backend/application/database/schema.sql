-- =============================================================================
-- Hotel Booking System — Database Schema
-- =============================================================================
-- Cascade rules:
--   chain deleted  → hotels deleted  → rooms deleted
--   hotel deleted  → rooms deleted
--   room deleted   → amenities/damages deleted; booking/renting room_id → NULL
--   customer deleted               → booking/renting customer_id → NULL
--   employee deleted               → renting employee_id → NULL; hotel manager_id → NULL
-- Archive preservation:
--   Bookings and rentings retain snapshot columns so historical records remain
--   meaningful even after customers, rooms, or hotels are deleted.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1. USERS  (authentication — linked optionally to customers and employees)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
  id            INT UNSIGNED NOT NULL AUTO_INCREMENT,
  email         VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  role          ENUM('admin', 'staff', 'guest') NOT NULL DEFAULT 'guest',
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_users_email (email)
);

-- -----------------------------------------------------------------------------
-- 2. CHAINS
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS chains (
  id             INT UNSIGNED NOT NULL AUTO_INCREMENT,
  name           VARCHAR(255) NOT NULL,
  office_street  VARCHAR(255) NOT NULL,
  office_city    VARCHAR(100) NOT NULL,
  office_state   VARCHAR(100) NOT NULL,
  office_country VARCHAR(100) NOT NULL,
  office_zip     VARCHAR(20)  NOT NULL,
  created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id)
  -- number_of_hotels is derived: SELECT COUNT(*) FROM hotels WHERE chain_id = ?
);

-- -----------------------------------------------------------------------------
-- 3. CHAIN CONTACTS  (multi-valued: emails + phones)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS chain_contacts (
  id       INT UNSIGNED                NOT NULL AUTO_INCREMENT,
  chain_id INT UNSIGNED                NOT NULL,
  type     ENUM('email', 'phone')      NOT NULL,
  value    VARCHAR(255)                NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_chain_contacts_chain
    FOREIGN KEY (chain_id) REFERENCES chains (id) ON DELETE CASCADE
);

-- -----------------------------------------------------------------------------
-- 4. EMPLOYEES
-- (created before hotels to resolve the manager FK — see hotels table below)
-- Hotel assignment is tracked in employee_positions (avoids circular FK).
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS employees (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id    INT UNSIGNED NULL,
  first_name VARCHAR(100) NOT NULL,
  last_name  VARCHAR(100) NOT NULL,
  street     VARCHAR(255) NOT NULL,
  city       VARCHAR(100) NOT NULL,
  state      VARCHAR(100) NOT NULL,
  country    VARCHAR(100) NOT NULL,
  zip        VARCHAR(20)  NOT NULL,
  ssn        VARCHAR(50)  NOT NULL,
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_employees_ssn (ssn),
  CONSTRAINT fk_employees_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);

-- -----------------------------------------------------------------------------
-- 5. HOTELS
-- chain_id CASCADE: deleting a chain removes all its hotels (and their rooms).
-- manager_id SET NULL: deleting an employee clears the manager slot.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS hotels (
  id          INT UNSIGNED     NOT NULL AUTO_INCREMENT,
  chain_id    INT UNSIGNED     NOT NULL,
  manager_id  INT UNSIGNED     NULL,
  name        VARCHAR(255)     NOT NULL,
  star_rating TINYINT UNSIGNED NOT NULL,
  street      VARCHAR(255)     NOT NULL,
  city        VARCHAR(100)     NOT NULL,
  state       VARCHAR(100)     NOT NULL,
  country     VARCHAR(100)     NOT NULL,
  zip         VARCHAR(20)      NOT NULL,
  created_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT chk_hotels_star_rating CHECK (star_rating BETWEEN 1 AND 5),
  CONSTRAINT fk_hotels_chain
    FOREIGN KEY (chain_id)  REFERENCES chains    (id) ON DELETE CASCADE,
  CONSTRAINT fk_hotels_manager
    FOREIGN KEY (manager_id) REFERENCES employees (id) ON DELETE SET NULL
  -- number_of_rooms is derived: SELECT COUNT(*) FROM rooms WHERE hotel_id = ?
);

-- -----------------------------------------------------------------------------
-- 6. HOTEL CONTACTS  (multi-valued: emails + phones)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS hotel_contacts (
  id       INT UNSIGNED           NOT NULL AUTO_INCREMENT,
  hotel_id INT UNSIGNED           NOT NULL,
  type     ENUM('email', 'phone') NOT NULL,
  value    VARCHAR(255)           NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_hotel_contacts_hotel
    FOREIGN KEY (hotel_id) REFERENCES hotels (id) ON DELETE CASCADE
);

-- -----------------------------------------------------------------------------
-- 7. EMPLOYEE POSITIONS
-- An employee may hold multiple roles, possibly at different hotels over time.
-- hotel_id CASCADE: removing a hotel removes the position records for it.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS employee_positions (
  id          INT UNSIGNED NOT NULL AUTO_INCREMENT,
  employee_id INT UNSIGNED NOT NULL,
  hotel_id    INT UNSIGNED NOT NULL,
  position    VARCHAR(100) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_employee_positions_employee
    FOREIGN KEY (employee_id) REFERENCES employees (id) ON DELETE CASCADE,
  CONSTRAINT fk_employee_positions_hotel
    FOREIGN KEY (hotel_id) REFERENCES hotels (id) ON DELETE CASCADE
);

-- -----------------------------------------------------------------------------
-- 8. ROOMS
-- hotel_id CASCADE: deleting a hotel removes all its rooms.
-- capacity replaces the old 'type' column; 'view' and 'extendable' are new.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rooms (
  id              INT UNSIGNED   NOT NULL AUTO_INCREMENT,
  hotel_id        INT UNSIGNED   NOT NULL,
  room_number     VARCHAR(20)    NOT NULL,
  price_per_night DECIMAL(10, 2) NOT NULL,
  capacity        ENUM('single', 'double', 'triple', 'quad') NOT NULL,
  view            ENUM('none', 'sea', 'mountain', 'city')    NOT NULL DEFAULT 'none',
  extendable      TINYINT(1)     NOT NULL DEFAULT 0,
  status          ENUM('available', 'occupied', 'maintenance') NOT NULL DEFAULT 'available',
  created_at      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_rooms_hotel_number (hotel_id, room_number),
  CONSTRAINT fk_rooms_hotel
    FOREIGN KEY (hotel_id) REFERENCES hotels (id) ON DELETE CASCADE
);

-- -----------------------------------------------------------------------------
-- 9. ROOM AMENITIES  (multi-valued: TV, AC, fridge, etc.)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS room_amenities (
  id      INT UNSIGNED NOT NULL AUTO_INCREMENT,
  room_id INT UNSIGNED NOT NULL,
  amenity VARCHAR(100) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_room_amenities_room
    FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE
);

-- -----------------------------------------------------------------------------
-- 10. ROOM DAMAGES  (multi-valued: current problems / damage reports)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS room_damages (
  id          INT UNSIGNED NOT NULL AUTO_INCREMENT,
  room_id     INT UNSIGNED NOT NULL,
  description TEXT         NOT NULL,
  reported_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved    TINYINT(1)   NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  CONSTRAINT fk_room_damages_room
    FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE
);

-- -----------------------------------------------------------------------------
-- 11. CUSTOMERS
-- user_id links to the login account (nullable: walk-in customers have none).
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS customers (
  id            INT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id       INT UNSIGNED NULL,
  first_name    VARCHAR(100) NOT NULL,
  last_name     VARCHAR(100) NOT NULL,
  street        VARCHAR(255) NOT NULL,
  city          VARCHAR(100) NOT NULL,
  state         VARCHAR(100) NOT NULL,
  country       VARCHAR(100) NOT NULL,
  zip           VARCHAR(20)  NOT NULL,
  id_type       ENUM('SSN', 'SIN', 'driving_licence') NOT NULL,
  id_number     VARCHAR(100) NOT NULL,
  registered_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_customers_identity (id_type, id_number),
  CONSTRAINT fk_customers_user
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);

-- -----------------------------------------------------------------------------
-- 12. BOOKINGS  (online reservations)
-- customer_id / room_id SET NULL on delete — history is preserved via snapshots.
-- Snapshot columns capture the state at time of booking for the archive.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bookings (
  id                     INT UNSIGNED   NOT NULL AUTO_INCREMENT,
  customer_id            INT UNSIGNED   NULL,
  room_id                INT UNSIGNED   NULL,
  -- Archive snapshots (remain after customer/room/hotel is deleted)
  snapshot_customer_name VARCHAR(255)   NOT NULL,
  snapshot_hotel_name    VARCHAR(255)   NOT NULL,
  snapshot_room_number   VARCHAR(20)    NOT NULL,
  snapshot_room_capacity VARCHAR(50)    NOT NULL,
  snapshot_price         DECIMAL(10, 2) NOT NULL,
  -- Booking window
  check_in_date          DATE           NOT NULL,
  check_out_date         DATE           NOT NULL,
  status                 ENUM('active', 'cancelled', 'checked_in') NOT NULL DEFAULT 'active',
  created_at             DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT fk_bookings_customer
    FOREIGN KEY (customer_id) REFERENCES customers (id) ON DELETE SET NULL,
  CONSTRAINT fk_bookings_room
    FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE SET NULL
);

-- -----------------------------------------------------------------------------
-- 13. RENTINGS  (active stays + historical archive)
-- booking_id NULL  = walk-in renting (no prior booking).
-- employee_id      = staff member who performed check-in.
-- All FK targets use SET NULL so the renting row is never deleted by cascade.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rentings (
  id                     INT UNSIGNED   NOT NULL AUTO_INCREMENT,
  booking_id             INT UNSIGNED   NULL,
  customer_id            INT UNSIGNED   NULL,
  room_id                INT UNSIGNED   NULL,
  employee_id            INT UNSIGNED   NULL,
  -- Archive snapshots
  snapshot_customer_name VARCHAR(255)   NOT NULL,
  snapshot_hotel_name    VARCHAR(255)   NOT NULL,
  snapshot_room_number   VARCHAR(20)    NOT NULL,
  snapshot_room_capacity VARCHAR(50)    NOT NULL,
  snapshot_price         DECIMAL(10, 2) NOT NULL,
  -- Stay window (check_out_date NULL while active)
  check_in_date          DATE           NOT NULL,
  check_out_date         DATE           NULL,
  total_amount           DECIMAL(10, 2) NULL,
  status                 ENUM('active', 'completed') NOT NULL DEFAULT 'active',
  created_at             DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  CONSTRAINT fk_rentings_booking
    FOREIGN KEY (booking_id)  REFERENCES bookings  (id) ON DELETE SET NULL,
  CONSTRAINT fk_rentings_customer
    FOREIGN KEY (customer_id) REFERENCES customers (id) ON DELETE SET NULL,
  CONSTRAINT fk_rentings_room
    FOREIGN KEY (room_id)     REFERENCES rooms     (id) ON DELETE SET NULL,
  CONSTRAINT fk_rentings_employee
    FOREIGN KEY (employee_id) REFERENCES employees (id) ON DELETE SET NULL
);

-- -----------------------------------------------------------------------------
-- 14. PAYMENTS  (no history required — deleted with renting)
-- One payment record per renting (enforced by UNIQUE KEY).
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS payments (
  id             INT UNSIGNED   NOT NULL AUTO_INCREMENT,
  renting_id     INT UNSIGNED   NOT NULL,
  amount         DECIMAL(10, 2) NOT NULL,
  payment_method ENUM('cash', 'credit_card', 'debit_card', 'bank_transfer') NOT NULL,
  paid_at        DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_payments_renting (renting_id),
  CONSTRAINT fk_payments_renting
    FOREIGN KEY (renting_id) REFERENCES rentings (id) ON DELETE CASCADE
);

-- -----------------------------------------------------------------------------
-- 15. REVIEWS  (one review per user per hotel)
-- hotel_id / user_id use SET NULL so historical reviews survive deletions.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS reviews (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  hotel_id   INT UNSIGNED NULL,
  user_id    INT UNSIGNED NULL,
  rating     TINYINT UNSIGNED NOT NULL,
  comment    TEXT         NULL,
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_reviews_hotel_user (hotel_id, user_id),
  CONSTRAINT chk_reviews_rating CHECK (rating BETWEEN 1 AND 5),
  CONSTRAINT fk_reviews_hotel
    FOREIGN KEY (hotel_id) REFERENCES hotels (id) ON DELETE SET NULL,
  CONSTRAINT fk_reviews_user
    FOREIGN KEY (user_id)  REFERENCES users  (id) ON DELETE SET NULL
);

-- =============================================================================
-- 16. INDEXES
-- =============================================================================
-- Three explicit performance indexes chosen based on the query patterns visible
-- in the repository layer (hotels/repository.go, rooms/repository.go,
-- bookings/repository.go, rentings/repository.go).

-- idx_hotels_chain_id
-- Justification: nearly every hotel fetch filters or joins on chain_id.
-- chains.FindByID uses LEFT JOIN hotels h ON h.chain_id = c.id, and
-- hotels.Search appends AND h.chain_id = ? for chain-scoped searches.
-- Without this index both queries would full-scan the hotels table for every
-- chain lookup.  Expected access pattern: frequent read (every chain page
-- load) with infrequent writes (hotels are rarely reassigned to another chain).
CREATE INDEX idx_hotels_chain_id ON hotels (chain_id);

-- idx_rooms_hotel_status
-- Justification: the most common room query pattern is "available rooms in
-- hotel X", fired by rooms.Search (AND r.hotel_id = ? AND r.status = ?) and
-- by the EXISTS availability sub-select inside hotels.Search.  A composite
-- index on (hotel_id, status) satisfies both equality filters in one B-tree
-- scan and avoids a full rooms-table scan for every availability check.
-- Expected access pattern: very frequent reads (every search request),
-- moderate writes (status changes on check-in / check-out events).
CREATE INDEX idx_rooms_hotel_status ON rooms (hotel_id, status);

-- idx_bookings_room_dates
-- Justification: bookings.Create and rentings.CreateWalkIn both execute an
-- overlap check of the form:
--   WHERE room_id = ? AND status = 'active'
--         AND check_in_date < ? AND check_out_date > ?
-- The leading room_id column reduces the scan to one room's bookings; the
-- date columns then allow the engine to prune non-overlapping intervals with
-- a range scan rather than a full table scan.  This index is critical for
-- correctness-under-load: slow overlap checks invite race conditions.
CREATE INDEX idx_bookings_room_dates ON bookings (room_id, check_in_date, check_out_date);

-- =============================================================================
-- 17. VIEWS
-- =============================================================================

-- View 1 — available_rooms_per_area
-- The number of rooms currently in 'available' status, grouped by geographic
-- area (city + state + country).  Useful for front-desk dashboards and for
-- showcasing GROUP BY aggregation across the hotels/rooms join.
CREATE OR REPLACE VIEW available_rooms_per_area AS
SELECT
    h.city,
    h.state,
    h.country,
    COUNT(r.id) AS available_rooms
FROM rooms r
JOIN hotels h ON h.id = r.hotel_id
WHERE r.status = 'available'
GROUP BY h.city, h.state, h.country
ORDER BY h.country, h.state, h.city;

-- View 2 — hotel_room_capacity
-- The aggregated guest capacity of all rooms for each hotel, computed by
-- mapping the capacity ENUM (single/double/triple/quad) to its numeric guest
-- count and summing.  Answers "how many guests can Hotel X accommodate at
-- full occupancy?" and demonstrates CASE-expression aggregation.
CREATE OR REPLACE VIEW hotel_room_capacity AS
SELECT
    h.id        AS hotel_id,
    h.name      AS hotel_name,
    c.name      AS chain_name,
    COUNT(r.id) AS total_rooms,
    SUM(
        CASE r.capacity
            WHEN 'single' THEN 1
            WHEN 'double' THEN 2
            WHEN 'triple' THEN 3
            WHEN 'quad'   THEN 4
            ELSE 0
        END
    ) AS total_capacity
FROM hotels h
JOIN chains c ON c.id = h.chain_id
LEFT JOIN rooms r ON r.hotel_id = h.id
GROUP BY h.id, h.name, c.name
ORDER BY h.id;

-- =============================================================================
-- 18. TRIGGERS
-- =============================================================================

-- Trigger 1 — trg_rentings_after_insert
-- After a new renting (check-in) is recorded, automatically mark the room as
-- 'occupied'.  This enforces the user-defined invariant that a room under an
-- active renting must never appear as 'available', even when the database is
-- accessed directly (e.g. admin tooling, data migrations) rather than through
-- the application layer.
DELIMITER //
CREATE TRIGGER trg_rentings_after_insert
AFTER INSERT ON rentings
FOR EACH ROW
BEGIN
    IF NEW.room_id IS NOT NULL THEN
        UPDATE rooms
        SET    status = 'occupied'
        WHERE  id = NEW.room_id;
    END IF;
END//
DELIMITER ;

-- Trigger 2 — trg_rentings_after_update
-- After a renting is completed (status transitions from 'active' to
-- 'completed'), automatically restore the room's status to 'available'.
-- This closes the occupancy cycle opened by Trigger 1 and guarantees that
-- room availability is always consistent with the rentings ledger, regardless
-- of which code path (application, direct SQL, future batch jobs) performs
-- the check-out.
DELIMITER //
CREATE TRIGGER trg_rentings_after_update
AFTER UPDATE ON rentings
FOR EACH ROW
BEGIN
    IF NEW.status = 'completed'
       AND OLD.status = 'active'
       AND NEW.room_id IS NOT NULL
    THEN
        UPDATE rooms
        SET    status = 'available'
        WHERE  id = NEW.room_id;
    END IF;
END//
DELIMITER ;
