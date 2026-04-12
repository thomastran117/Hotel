package rentings

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
	ErrRentingNotFound      = errors.New("rentings: renting not found")
	ErrRoomNotFound         = errors.New("rentings: room not found")
	ErrCustomerNotFound     = errors.New("rentings: customer not found")
	ErrEmployeeNotFound     = errors.New("rentings: employee profile not found for this user")
	ErrRoomOccupied         = errors.New("rentings: room is already occupied or reserved for this date")
	ErrBookingNotFound      = errors.New("rentings: booking not found")
	ErrBookingNotActive     = errors.New("rentings: booking is not in active status")
	ErrAlreadyCheckedOut    = errors.New("rentings: renting is already completed")
	ErrNotOwner             = errors.New("rentings: not the owner of this renting")
	ErrInvalidCheckout      = errors.New("rentings: check-out date must be after check-in date")
	ErrInvalidPaymentMethod = errors.New("rentings: invalid payment method")
)

// validPaymentMethods is the set of accepted payment_method values.
var validPaymentMethods = map[string]struct{}{
	"cash": {}, "credit_card": {}, "debit_card": {}, "bank_transfer": {},
}

// ── Domain types ──────────────────────────────────────────────────────────────

// Payment mirrors one row in the payments table.
type Payment struct {
	ID            uint      `json:"id"`
	RentingID     uint      `json:"renting_id"`
	Amount        float64   `json:"amount"`
	PaymentMethod string    `json:"payment_method"`
	PaidAt        time.Time `json:"paid_at"`
}

// Renting mirrors one row in the rentings table, with an optional embedded payment.
type Renting struct {
	ID                   uint       `json:"id"`
	BookingID            *uint      `json:"booking_id"`
	CustomerID           *uint      `json:"customer_id"`
	RoomID               *uint      `json:"room_id"`
	EmployeeID           *uint      `json:"employee_id"`
	SnapshotCustomerName string     `json:"snapshot_customer_name"`
	SnapshotHotelName    string     `json:"snapshot_hotel_name"`
	SnapshotRoomNumber   string     `json:"snapshot_room_number"`
	SnapshotRoomCapacity string     `json:"snapshot_room_capacity"`
	SnapshotPrice        float64    `json:"snapshot_price"`
	CheckInDate          time.Time  `json:"check_in_date"`
	CheckOutDate         *time.Time `json:"check_out_date"`
	TotalAmount          *float64   `json:"total_amount"`
	Status               string     `json:"status"`
	CreatedAt            time.Time  `json:"created_at"`
	Payment              *Payment   `json:"payment"`
}

// ── Input / filter types ──────────────────────────────────────────────────────

// WalkInInput carries all fields needed to create a walk-in renting.
type WalkInInput struct {
	CustomerID  uint
	RoomID      uint
	EmployeeID  uint
	CheckInDate string // YYYY-MM-DD
}

// RentingFilter carries optional parameters for the List method.
type RentingFilter struct {
	CustomerID *uint
	Status     *string
	Limit      int
	Offset     int
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the rentings feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// FindEmployeeIDByUserID looks up the employee profile for a given user account.
func (r *Repository) FindEmployeeIDByUserID(ctx context.Context, userID uint) (uint, error) {
	var id uint
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM employees WHERE user_id = ? LIMIT 1`, userID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrEmployeeNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("rentings: find employee by user id: %w", err)
	}
	return id, nil
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
		return 0, fmt.Errorf("rentings: find customer by user id: %w", err)
	}
	return id, nil
}

// CreateWalkIn inserts a walk-in renting inside a transaction.
// It locks the room row, verifies no active renting exists and no booking covers
// the check-in date, then commits.
func (r *Repository) CreateWalkIn(ctx context.Context, in WalkInInput) (*Renting, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rentings: begin tx: %w", err)
	}

	// Lock the room and collect snapshot data.
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
		return nil, fmt.Errorf("rentings: lock room: %w", err)
	}

	// Verify no active renting already occupies this room.
	var count int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM rentings
		WHERE room_id = ? AND status = 'active'`,
		in.RoomID,
	).Scan(&count); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: check active renting: %w", err)
	}
	if count > 0 {
		tx.Rollback()
		return nil, ErrRoomOccupied
	}

	// Verify no active booking covers the requested check-in date.
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM bookings
		WHERE room_id = ? AND status = 'active'
		  AND check_in_date <= ? AND check_out_date > ?`,
		in.RoomID, in.CheckInDate, in.CheckInDate,
	).Scan(&count); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: check booking conflict: %w", err)
	}
	if count > 0 {
		tx.Rollback()
		return nil, ErrRoomOccupied
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
		return nil, fmt.Errorf("rentings: get customer: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO rentings
			(customer_id, room_id, employee_id,
			 snapshot_customer_name, snapshot_hotel_name,
			 snapshot_room_number, snapshot_room_capacity, snapshot_price,
			 check_in_date, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		in.CustomerID, in.RoomID, in.EmployeeID,
		firstName+" "+lastName, hotelName,
		roomNumber, roomCapacity, price,
		in.CheckInDate,
	)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: insert walk-in: %w", err)
	}

	id64, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: last insert id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("rentings: commit: %w", err)
	}

	return r.FindByID(ctx, uint(id64))
}

// CreateFromBooking atomically converts an active booking into a renting.
// It locks the booking row, verifies its status, checks for existing active
// rentings on the room, marks the booking as checked_in, and inserts the renting.
func (r *Repository) CreateFromBooking(ctx context.Context, bookingID, employeeID uint) (*Renting, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rentings: begin tx: %w", err)
	}

	// Lock the booking row and read all data needed for the renting snapshot.
	var (
		bCustomerID           *uint
		bRoomID               *uint
		bCustomerName         string
		bHotelName            string
		bRoomNumber           string
		bRoomCapacity         string
		bPrice                float64
		bCheckInDate          time.Time
		bStatus               string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT customer_id, room_id,
		       snapshot_customer_name, snapshot_hotel_name,
		       snapshot_room_number, snapshot_room_capacity, snapshot_price,
		       check_in_date, status
		FROM bookings WHERE id = ?
		FOR UPDATE`,
		bookingID,
	).Scan(
		&bCustomerID, &bRoomID,
		&bCustomerName, &bHotelName, &bRoomNumber, &bRoomCapacity, &bPrice,
		&bCheckInDate, &bStatus,
	)
	if errors.Is(err, sql.ErrNoRows) {
		tx.Rollback()
		return nil, ErrBookingNotFound
	}
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: lock booking: %w", err)
	}

	if bStatus != "active" {
		tx.Rollback()
		return nil, ErrBookingNotActive
	}

	// Verify no active renting already occupies the room.
	if bRoomID != nil {
		var count int
		if err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM rentings
			WHERE room_id = ? AND status = 'active'`,
			*bRoomID,
		).Scan(&count); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("rentings: check active renting: %w", err)
		}
		if count > 0 {
			tx.Rollback()
			return nil, ErrRoomOccupied
		}
	}

	// Mark the booking as checked in.
	if _, err = tx.ExecContext(ctx,
		`UPDATE bookings SET status = 'checked_in' WHERE id = ?`, bookingID,
	); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: update booking status: %w", err)
	}

	// Insert the renting using the booking's snapshot data.
	result, err := tx.ExecContext(ctx, `
		INSERT INTO rentings
			(booking_id, customer_id, room_id, employee_id,
			 snapshot_customer_name, snapshot_hotel_name,
			 snapshot_room_number, snapshot_room_capacity, snapshot_price,
			 check_in_date, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		bookingID, bCustomerID, bRoomID, employeeID,
		bCustomerName, bHotelName, bRoomNumber, bRoomCapacity, bPrice,
		bCheckInDate,
	)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: insert from booking: %w", err)
	}

	id64, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: last insert id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("rentings: commit: %w", err)
	}

	return r.FindByID(ctx, uint(id64))
}

// FindByID retrieves a renting by primary key, including its payment if present.
// Returns ErrRentingNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Renting, error) {
	rt := &Renting{}
	var (
		payID     sql.NullInt64
		payAmount sql.NullFloat64
		payMethod sql.NullString
		payAt     sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT rt.id, rt.booking_id, rt.customer_id, rt.room_id, rt.employee_id,
		       rt.snapshot_customer_name, rt.snapshot_hotel_name,
		       rt.snapshot_room_number, rt.snapshot_room_capacity, rt.snapshot_price,
		       rt.check_in_date, rt.check_out_date, rt.total_amount, rt.status, rt.created_at,
		       p.id, p.amount, p.payment_method, p.paid_at
		FROM rentings rt
		LEFT JOIN payments p ON p.renting_id = rt.id
		WHERE rt.id = ?`, id,
	).Scan(
		&rt.ID, &rt.BookingID, &rt.CustomerID, &rt.RoomID, &rt.EmployeeID,
		&rt.SnapshotCustomerName, &rt.SnapshotHotelName,
		&rt.SnapshotRoomNumber, &rt.SnapshotRoomCapacity, &rt.SnapshotPrice,
		&rt.CheckInDate, &rt.CheckOutDate, &rt.TotalAmount, &rt.Status, &rt.CreatedAt,
		&payID, &payAmount, &payMethod, &payAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRentingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("rentings: find by id: %w", err)
	}
	if payID.Valid {
		rt.Payment = &Payment{
			ID:            uint(payID.Int64),
			RentingID:     rt.ID,
			Amount:        payAmount.Float64,
			PaymentMethod: payMethod.String,
			PaidAt:        payAt.Time,
		}
	}
	return rt, nil
}

// List returns rentings matching the filter ordered by most recent first,
// including each renting's payment if present.
func (r *Repository) List(ctx context.Context, f RentingFilter) ([]*Renting, error) {
	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT rt.id, rt.booking_id, rt.customer_id, rt.room_id, rt.employee_id,
		       rt.snapshot_customer_name, rt.snapshot_hotel_name,
		       rt.snapshot_room_number, rt.snapshot_room_capacity, rt.snapshot_price,
		       rt.check_in_date, rt.check_out_date, rt.total_amount, rt.status, rt.created_at,
		       p.id, p.amount, p.payment_method, p.paid_at
		FROM rentings rt
		LEFT JOIN payments p ON p.renting_id = rt.id
		WHERE 1=1`)

	if f.CustomerID != nil {
		sb.WriteString(" AND rt.customer_id = ?")
		args = append(args, *f.CustomerID)
	}
	if f.Status != nil {
		sb.WriteString(" AND rt.status = ?")
		args = append(args, *f.Status)
	}

	sb.WriteString(" ORDER BY rt.created_at DESC LIMIT ? OFFSET ?")
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("rentings: list: %w", err)
	}
	defer rows.Close()

	var rentings []*Renting
	for rows.Next() {
		rt := &Renting{}
		var (
			payID     sql.NullInt64
			payAmount sql.NullFloat64
			payMethod sql.NullString
			payAt     sql.NullTime
		)
		if err := rows.Scan(
			&rt.ID, &rt.BookingID, &rt.CustomerID, &rt.RoomID, &rt.EmployeeID,
			&rt.SnapshotCustomerName, &rt.SnapshotHotelName,
			&rt.SnapshotRoomNumber, &rt.SnapshotRoomCapacity, &rt.SnapshotPrice,
			&rt.CheckInDate, &rt.CheckOutDate, &rt.TotalAmount, &rt.Status, &rt.CreatedAt,
			&payID, &payAmount, &payMethod, &payAt,
		); err != nil {
			return nil, fmt.Errorf("rentings: scan: %w", err)
		}
		if payID.Valid {
			rt.Payment = &Payment{
				ID:            uint(payID.Int64),
				RentingID:     rt.ID,
				Amount:        payAmount.Float64,
				PaymentMethod: payMethod.String,
				PaidAt:        payAt.Time,
			}
		}
		rentings = append(rentings, rt)
	}
	if rentings == nil {
		rentings = []*Renting{}
	}
	return rentings, rows.Err()
}

// CheckOut completes a renting: it sets check_out_date and total_amount, marks
// the renting as completed, and records the payment — all in one transaction.
func (r *Repository) CheckOut(ctx context.Context, id uint, checkOutDate, paymentMethod string) (*Renting, error) {
	if _, ok := validPaymentMethods[paymentMethod]; !ok {
		return nil, ErrInvalidPaymentMethod
	}

	rt, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rt.Status == "completed" {
		return nil, ErrAlreadyCheckedOut
	}

	checkOut, err := time.Parse("2006-01-02", checkOutDate)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid date format", ErrInvalidCheckout)
	}

	// Truncate both to date-only for reliable night calculation.
	checkIn := rt.CheckInDate.Truncate(24 * time.Hour)
	checkOut = checkOut.Truncate(24 * time.Hour)
	nights := int(checkOut.Sub(checkIn).Hours() / 24)
	if nights < 1 {
		return nil, ErrInvalidCheckout
	}

	totalAmount := rt.SnapshotPrice * float64(nights)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rentings: begin checkout tx: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE rentings
		SET check_out_date = ?, total_amount = ?, status = 'completed'
		WHERE id = ?`,
		checkOutDate, totalAmount, id,
	); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: update for checkout: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO payments (renting_id, amount, payment_method)
		VALUES (?, ?, ?)`,
		id, totalAmount, paymentMethod,
	); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rentings: insert payment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("rentings: commit checkout: %w", err)
	}

	return r.FindByID(ctx, id)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func isForeignKeyViolation(err error) bool {
	return strings.Contains(err.Error(), "a foreign key constraint fails")
}
