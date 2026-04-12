package rooms

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
	ErrRoomNotFound   = errors.New("rooms: room not found")
	ErrHotelNotFound  = errors.New("rooms: hotel not found")
	ErrRoomNumberTaken = errors.New("rooms: room number already exists in this hotel")
)

// ── Domain types ──────────────────────────────────────────────────────────────

// Amenity represents one entry in the room_amenities table.
type Amenity struct {
	ID      uint   `json:"id"`
	RoomID  uint   `json:"room_id"`
	Amenity string `json:"amenity"`
}

// Damage represents one entry in the room_damages table.
type Damage struct {
	ID          uint      `json:"id"`
	RoomID      uint      `json:"room_id"`
	Description string    `json:"description"`
	ReportedAt  time.Time `json:"reported_at"`
	Resolved    bool      `json:"resolved"`
}

// Room mirrors the rooms table with its related amenities and damages.
type Room struct {
	ID            uint      `json:"id"`
	HotelID       uint      `json:"hotel_id"`
	RoomNumber    string    `json:"room_number"`
	PricePerNight float64   `json:"price_per_night"`
	Capacity      string    `json:"capacity"`
	View          string    `json:"view"`
	Extendable    bool      `json:"extendable"`
	Status        string    `json:"status"`
	Amenities     []Amenity `json:"amenities"`
	Damages       []Damage  `json:"damages"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ── Input types ───────────────────────────────────────────────────────────────

// DamageInput carries the fields needed to create a damage record.
type DamageInput struct {
	Description string
	Resolved    bool
}

// RoomInput carries all mutable fields for create and update.
type RoomInput struct {
	HotelID       uint
	RoomNumber    string
	PricePerNight float64
	Capacity      string
	View          string
	Extendable    bool
	Status        string
	Amenities     []string
	Damages       []DamageInput
}

// RoomFilter carries optional search parameters for the Search method.
type RoomFilter struct {
	HotelID    *uint
	ChainName  *string // LIKE search against chains.name
	RoomNumber *string
	Capacity   *string
	View       *string
	MinPrice   *float64
	MaxPrice   *float64
	Status     *string
	Extendable *bool
	CheckIn    *string // "YYYY-MM-DD"
	CheckOut   *string // "YYYY-MM-DD"
	Limit      int
	Offset     int
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the rooms feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new room with its amenities and damages in a single transaction.
func (r *Repository) Create(ctx context.Context, in RoomInput) (*Room, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rooms: begin tx: %w", err)
	}

	const q = `
		INSERT INTO rooms (hotel_id, room_number, price_per_night, capacity, view, extendable, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := tx.ExecContext(ctx, q,
		in.HotelID, in.RoomNumber, in.PricePerNight,
		in.Capacity, in.View, in.Extendable, in.Status,
	)
	if err != nil {
		tx.Rollback()
		if isForeignKeyViolation(err) {
			return nil, ErrHotelNotFound
		}
		if isDuplicateEntry(err) {
			return nil, ErrRoomNumberTaken
		}
		return nil, fmt.Errorf("rooms: insert room: %w", err)
	}
	id64, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rooms: last insert id: %w", err)
	}
	roomID := uint(id64)

	if err := insertAmenities(ctx, tx, roomID, in.Amenities); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := insertDamages(ctx, tx, roomID, in.Damages); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("rooms: commit tx: %w", err)
	}

	return r.FindByID(ctx, roomID)
}

// FindByID retrieves a room by primary key. Returns ErrRoomNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Room, error) {
	const q = `
		SELECT id, hotel_id, room_number, price_per_night, capacity, view, extendable, status, created_at, updated_at
		FROM rooms
		WHERE id = ?`

	room := &Room{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&room.ID, &room.HotelID, &room.RoomNumber, &room.PricePerNight,
		&room.Capacity, &room.View, &room.Extendable, &room.Status,
		&room.CreatedAt, &room.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRoomNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("rooms: find by id: %w", err)
	}

	amenities, err := r.loadAmenities(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	room.Amenities = amenities

	damages, err := r.loadDamages(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	room.Damages = damages

	return room, nil
}

// Search returns rooms matching the given filter with pagination.
// When CheckIn/CheckOut are set (always, via service defaults), rooms with
// overlapping active bookings or open rentings are excluded.
func (r *Repository) Search(ctx context.Context, f RoomFilter) ([]*Room, error) {
	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT r.id, r.hotel_id, r.room_number, r.price_per_night, r.capacity, r.view, r.extendable, r.status, r.created_at, r.updated_at
		FROM rooms r
		JOIN hotels h ON h.id = r.hotel_id
		JOIN chains c ON c.id = h.chain_id
		WHERE 1=1`)

	if f.HotelID != nil {
		sb.WriteString(" AND r.hotel_id = ?")
		args = append(args, *f.HotelID)
	}
	if f.ChainName != nil {
		sb.WriteString(" AND c.name LIKE ?")
		args = append(args, "%"+*f.ChainName+"%")
	}
	if f.RoomNumber != nil {
		sb.WriteString(" AND r.room_number = ?")
		args = append(args, *f.RoomNumber)
	}
	if f.Capacity != nil {
		sb.WriteString(" AND r.capacity = ?")
		args = append(args, *f.Capacity)
	}
	if f.View != nil {
		sb.WriteString(" AND r.view = ?")
		args = append(args, *f.View)
	}
	if f.MinPrice != nil {
		sb.WriteString(" AND r.price_per_night >= ?")
		args = append(args, *f.MinPrice)
	}
	if f.MaxPrice != nil {
		sb.WriteString(" AND r.price_per_night <= ?")
		args = append(args, *f.MaxPrice)
	}
	if f.Status != nil {
		sb.WriteString(" AND r.status = ?")
		args = append(args, *f.Status)
	}
	if f.Extendable != nil {
		sb.WriteString(" AND r.extendable = ?")
		args = append(args, *f.Extendable)
	}

	// Availability: exclude rooms with an overlapping active booking.
	if f.CheckIn != nil && f.CheckOut != nil {
		sb.WriteString(`
		AND r.id NOT IN (
			SELECT b.room_id FROM bookings b
			WHERE b.status = 'active'
			  AND b.room_id IS NOT NULL
			  AND b.check_in_date < ? AND b.check_out_date > ?
		)`)
		args = append(args, *f.CheckOut, *f.CheckIn)

		// Exclude rooms with an overlapping active renting.
		// check_out_date IS NULL means the guest is still checked in with no known end.
		sb.WriteString(`
		AND r.id NOT IN (
			SELECT rt.room_id FROM rentings rt
			WHERE rt.status = 'active'
			  AND rt.room_id IS NOT NULL
			  AND rt.check_in_date < ?
			  AND (rt.check_out_date IS NULL OR rt.check_out_date > ?)
		)`)
		args = append(args, *f.CheckOut, *f.CheckIn)
	}

	sb.WriteString(" ORDER BY r.id")
	sb.WriteString(" LIMIT ? OFFSET ?")
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("rooms: search: %w", err)
	}
	defer rows.Close()

	var rooms []*Room
	var ids []uint
	for rows.Next() {
		rm := &Room{}
		if err := rows.Scan(
			&rm.ID, &rm.HotelID, &rm.RoomNumber, &rm.PricePerNight,
			&rm.Capacity, &rm.View, &rm.Extendable, &rm.Status,
			&rm.CreatedAt, &rm.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("rooms: scan room: %w", err)
		}
		rm.Amenities = []Amenity{}
		rm.Damages = []Damage{}
		rooms = append(rooms, rm)
		ids = append(ids, rm.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rooms: rows: %w", err)
	}

	if len(ids) == 0 {
		return rooms, nil
	}

	amenityMap, err := r.loadAmenitiesForIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	damageMap, err := r.loadDamagesForIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, rm := range rooms {
		if a, ok := amenityMap[rm.ID]; ok {
			rm.Amenities = a
		}
		if d, ok := damageMap[rm.ID]; ok {
			rm.Damages = d
		}
	}
	return rooms, nil
}

// Update replaces room fields, amenities, and damages in a single transaction.
func (r *Repository) Update(ctx context.Context, id uint, in RoomInput) (*Room, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rooms: begin tx: %w", err)
	}

	const q = `
		UPDATE rooms
		SET hotel_id = ?, room_number = ?, price_per_night = ?,
		    capacity = ?, view = ?, extendable = ?, status = ?
		WHERE id = ?`
	result, err := tx.ExecContext(ctx, q,
		in.HotelID, in.RoomNumber, in.PricePerNight,
		in.Capacity, in.View, in.Extendable, in.Status, id,
	)
	if err != nil {
		tx.Rollback()
		if isForeignKeyViolation(err) {
			return nil, ErrHotelNotFound
		}
		if isDuplicateEntry(err) {
			return nil, ErrRoomNumberTaken
		}
		return nil, fmt.Errorf("rooms: update room: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rooms: rows affected: %w", err)
	}
	if n == 0 {
		tx.Rollback()
		return nil, ErrRoomNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM room_amenities WHERE room_id = ?`, id); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rooms: delete amenities: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM room_damages WHERE room_id = ?`, id); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("rooms: delete damages: %w", err)
	}

	if err := insertAmenities(ctx, tx, id, in.Amenities); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := insertDamages(ctx, tx, id, in.Damages); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("rooms: commit tx: %w", err)
	}

	return r.FindByID(ctx, id)
}

// Delete removes a room by primary key. Cascades to amenities/damages via FK.
func (r *Repository) Delete(ctx context.Context, id uint) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM rooms WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("rooms: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rooms: rows affected: %w", err)
	}
	if n == 0 {
		return ErrRoomNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func insertAmenities(ctx context.Context, tx *sql.Tx, roomID uint, amenities []string) error {
	if len(amenities) == 0 {
		return nil
	}
	const q = `INSERT INTO room_amenities (room_id, amenity) VALUES (?, ?)`
	for _, a := range amenities {
		if _, err := tx.ExecContext(ctx, q, roomID, a); err != nil {
			return fmt.Errorf("rooms: insert amenity: %w", err)
		}
	}
	return nil
}

func insertDamages(ctx context.Context, tx *sql.Tx, roomID uint, damages []DamageInput) error {
	if len(damages) == 0 {
		return nil
	}
	const q = `INSERT INTO room_damages (room_id, description, resolved) VALUES (?, ?, ?)`
	for _, d := range damages {
		if _, err := tx.ExecContext(ctx, q, roomID, d.Description, d.Resolved); err != nil {
			return fmt.Errorf("rooms: insert damage: %w", err)
		}
	}
	return nil
}

func (r *Repository) loadAmenities(ctx context.Context, roomID uint) ([]Amenity, error) {
	const q = `SELECT id, room_id, amenity FROM room_amenities WHERE room_id = ? ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q, roomID)
	if err != nil {
		return nil, fmt.Errorf("rooms: load amenities: %w", err)
	}
	defer rows.Close()

	var amenities []Amenity
	for rows.Next() {
		var a Amenity
		if err := rows.Scan(&a.ID, &a.RoomID, &a.Amenity); err != nil {
			return nil, fmt.Errorf("rooms: scan amenity: %w", err)
		}
		amenities = append(amenities, a)
	}
	if amenities == nil {
		amenities = []Amenity{}
	}
	return amenities, rows.Err()
}

func (r *Repository) loadDamages(ctx context.Context, roomID uint) ([]Damage, error) {
	const q = `SELECT id, room_id, description, reported_at, resolved FROM room_damages WHERE room_id = ? ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q, roomID)
	if err != nil {
		return nil, fmt.Errorf("rooms: load damages: %w", err)
	}
	defer rows.Close()

	var damages []Damage
	for rows.Next() {
		var d Damage
		if err := rows.Scan(&d.ID, &d.RoomID, &d.Description, &d.ReportedAt, &d.Resolved); err != nil {
			return nil, fmt.Errorf("rooms: scan damage: %w", err)
		}
		damages = append(damages, d)
	}
	if damages == nil {
		damages = []Damage{}
	}
	return damages, rows.Err()
}

func (r *Repository) loadAmenitiesForIDs(ctx context.Context, ids []uint) (map[uint][]Amenity, error) {
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	q := fmt.Sprintf(`SELECT id, room_id, amenity FROM room_amenities WHERE room_id IN (%s) ORDER BY id`, placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("rooms: load amenities for ids: %w", err)
	}
	defer rows.Close()

	m := make(map[uint][]Amenity)
	for rows.Next() {
		var a Amenity
		if err := rows.Scan(&a.ID, &a.RoomID, &a.Amenity); err != nil {
			return nil, fmt.Errorf("rooms: scan amenity: %w", err)
		}
		m[a.RoomID] = append(m[a.RoomID], a)
	}
	return m, rows.Err()
}

func (r *Repository) loadDamagesForIDs(ctx context.Context, ids []uint) (map[uint][]Damage, error) {
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	q := fmt.Sprintf(`SELECT id, room_id, description, reported_at, resolved FROM room_damages WHERE room_id IN (%s) ORDER BY id`, placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("rooms: load damages for ids: %w", err)
	}
	defer rows.Close()

	m := make(map[uint][]Damage)
	for rows.Next() {
		var d Damage
		if err := rows.Scan(&d.ID, &d.RoomID, &d.Description, &d.ReportedAt, &d.Resolved); err != nil {
			return nil, fmt.Errorf("rooms: scan damage: %w", err)
		}
		m[d.RoomID] = append(m[d.RoomID], d)
	}
	return m, rows.Err()
}

// isForeignKeyViolation reports whether err is a MySQL FK constraint failure (1452).
func isForeignKeyViolation(err error) bool {
	return strings.Contains(err.Error(), "a foreign key constraint fails")
}

// isDuplicateEntry reports whether err is a MySQL duplicate key error (1062).
func isDuplicateEntry(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry")
}
