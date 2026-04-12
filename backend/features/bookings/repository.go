package bookings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrBookingNotFound  = errors.New("bookings: booking not found")
	ErrRoomNotFound     = errors.New("bookings: room not found")
	ErrCustomerNotFound = errors.New("bookings: customer profile not found")
	ErrRoomUnavailable  = errors.New("bookings: room is not available for the requested dates")
	ErrNotOwner         = errors.New("bookings: not the owner of this booking")
	ErrAlreadyCancelled = errors.New("bookings: booking is already cancelled")
	ErrAlreadyCheckedIn = errors.New("bookings: booking has already been checked in")
	ErrInvalidDates     = errors.New("bookings: check-out date must be after check-in date")
)

// ── Domain types ──────────────────────────────────────────────────────────────

// Booking mirrors one row in the bookings table.
type Booking struct {
	ID                   uint      `json:"id"`
	CustomerID           *uint     `json:"customer_id"`
	RoomID               *uint     `json:"room_id"`
	SnapshotCustomerName string    `json:"snapshot_customer_name"`
	SnapshotHotelName    string    `json:"snapshot_hotel_name"`
	SnapshotRoomNumber   string    `json:"snapshot_room_number"`
	SnapshotRoomCapacity string    `json:"snapshot_room_capacity"`
	SnapshotPrice        float64   `json:"snapshot_price"`
	CheckInDate          time.Time `json:"check_in_date"`
	CheckOutDate         time.Time `json:"check_out_date"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
}

// ── Input / filter types ──────────────────────────────────────────────────────

// BookingInput carries all fields required to create a booking.
type BookingInput struct {
	CustomerID   uint
	RoomID       uint
	CheckInDate  string // YYYY-MM-DD
	CheckOutDate string // YYYY-MM-DD
}

// BookingFilter carries optional parameters for the List method.
type BookingFilter struct {
	CustomerID *uint
	Status     *string
	Limit      int
	Offset     int
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the bookings feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// FindCustomerIDByUserID looks up the customer profile for a given user account.
func (r *Repository) FindCustomerIDByUserID(ctx context.Context, userID uint) (uint, error) {
	var id uint
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM customers WHERE user_id = ? LIMIT 1`, userID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrCustomerNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("bookings: find customer by user id: %w", err)
	}
	return id, nil
}

// Create inserts a new booking inside a transaction. It locks the room row,
// verifies no overlap with active bookings or rentings, then commits.
func (r *Repository) Create(ctx context.Context, in BookingInput) (*Booking, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("bookings: begin tx: %w", err)
	}

	// Lock the room and collect snapshot data in one query.
	var (
		roomNumber   string
		roomCapacity string
		price        float64
		hotelName    string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT r.room_number, r.capacity, r.price_per_night, h.name
		FROM rooms r
		JOIN hotels h ON h.id = r.hotel_id
		WHERE r.id = ?
		FOR UPDATE`,
		in.RoomID,
	).Scan(&roomNumber, &roomCapacity, &price, &hotelName)
	if errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return nil, ErrRoomNotFound
	}
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("bookings: lock room: %w", err)
	}

	// Check overlap: active bookings on the same room.
	var count int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM bookings
		WHERE room_id = ? AND status = 'active'
		  AND check_in_date < ? AND check_out_date > ?`,
		in.RoomID, in.CheckOutDate, in.CheckInDate,
	).Scan(&count); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("bookings: check booking overlap: %w", err)
	}
	if count > 0 {
		tx.Rollback()
		return nil, ErrRoomUnavailable
	}

	// Check overlap: active rentings on the same room (NULL checkout = open-ended stay).
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM rentings
		WHERE room_id = ? AND status = 'active'
		  AND check_in_date < ?
		  AND (check_out_date IS NULL OR check_out_date > ?)`,
		in.RoomID, in.CheckOutDate, in.CheckInDate,
	).Scan(&count); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("bookings: check renting overlap: %w", err)
	}
	if count > 0 {
		tx.Rollback()
		return nil, ErrRoomUnavailable
	}

	// Fetch customer name for the snapshot.
	var firstName, lastName string
	if err = tx.QueryRowContext(ctx,
		`SELECT first_name, last_name FROM customers WHERE id = ?`, in.CustomerID,
	).Scan(&firstName, &lastName); err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrCustomerNotFound
		}
		return nil, fmt.Errorf("bookings: get customer name: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO bookings
			(customer_id, room_id,
			 snapshot_customer_name, snapshot_hotel_name,
			 snapshot_room_number, snapshot_room_capacity, snapshot_price,
			 check_in_date, check_out_date, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		in.CustomerID, in.RoomID,
		firstName+" "+lastName, hotelName,
		roomNumber, roomCapacity, price,
		in.CheckInDate, in.CheckOutDate,
	)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("bookings: insert: %w", err)
	}

	id64, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("bookings: last insert id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("bookings: commit: %w", err)
	}

	return r.FindByID(ctx, uint(id64))
}

// FindByID retrieves a booking by primary key. Returns ErrBookingNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Booking, error) {
	b := &Booking{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, customer_id, room_id,
		       snapshot_customer_name, snapshot_hotel_name,
		       snapshot_room_number, snapshot_room_capacity, snapshot_price,
		       check_in_date, check_out_date, status, created_at
		FROM bookings WHERE id = ?`, id,
	).Scan(
		&b.ID, &b.CustomerID, &b.RoomID,
		&b.SnapshotCustomerName, &b.SnapshotHotelName,
		&b.SnapshotRoomNumber, &b.SnapshotRoomCapacity, &b.SnapshotPrice,
		&b.CheckInDate, &b.CheckOutDate, &b.Status, &b.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBookingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bookings: find by id: %w", err)
	}
	return b, nil
}

// List returns bookings matching the filter ordered by most recent first.
func (r *Repository) List(ctx context.Context, f BookingFilter) ([]*Booking, error) {
	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT id, customer_id, room_id,
		       snapshot_customer_name, snapshot_hotel_name,
		       snapshot_room_number, snapshot_room_capacity, snapshot_price,
		       check_in_date, check_out_date, status, created_at
		FROM bookings WHERE 1=1`)

	if f.CustomerID != nil {
		sb.WriteString(" AND customer_id = ?")
		args = append(args, *f.CustomerID)
	}
	if f.Status != nil {
		sb.WriteString(" AND status = ?")
		args = append(args, *f.Status)
	}

	sb.WriteString(" ORDER BY created_at DESC LIMIT ? OFFSET ?")
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("bookings: list: %w", err)
	}
	defer rows.Close()

	var bookings []*Booking
	for rows.Next() {
		b := &Booking{}
		if err := rows.Scan(
			&b.ID, &b.CustomerID, &b.RoomID,
			&b.SnapshotCustomerName, &b.SnapshotHotelName,
			&b.SnapshotRoomNumber, &b.SnapshotRoomCapacity, &b.SnapshotPrice,
			&b.CheckInDate, &b.CheckOutDate, &b.Status, &b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("bookings: scan: %w", err)
		}
		bookings = append(bookings, b)
	}
	if bookings == nil {
		bookings = []*Booking{}
	}
	return bookings, rows.Err()
}

// Cancel sets a booking's status to 'cancelled'.
// Returns a specific error if the booking is already cancelled or checked in.
func (r *Repository) Cancel(ctx context.Context, id uint) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE bookings SET status = 'cancelled' WHERE id = ? AND status = 'active'`, id)
	if err != nil {
		return fmt.Errorf("bookings: cancel: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("bookings: rows affected: %w", err)
	}
	if n == 0 {
		b, ferr := r.FindByID(ctx, id)
		if ferr != nil {
			return ErrBookingNotFound
		}
		if b.Status == "cancelled" {
			return ErrAlreadyCancelled
		}
		return ErrAlreadyCheckedIn
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func isForeignKeyViolation(err error) bool {
	return strings.Contains(err.Error(), "a foreign key constraint fails")
}
