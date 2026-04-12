package hotels

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
	ErrHotelNotFound = errors.New("hotels: hotel not found")
	ErrChainNotFound = errors.New("hotels: chain not found")
)

// ── Domain types ──────────────────────────────────────────────────────────────

// Contact represents one entry in the hotel_contacts table.
type Contact struct {
	ID    uint   `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Hotel mirrors the hotels table with derived fields.
type Hotel struct {
	ID            uint      `json:"id"`
	ChainID       uint      `json:"chain_id"`
	ManagerID     *uint     `json:"manager_id"`
	Name          string    `json:"name"`
	StarRating    int       `json:"star_rating"`
	Street        string    `json:"street"`
	City          string    `json:"city"`
	State         string    `json:"state"`
	Country       string    `json:"country"`
	Zip           string    `json:"zip"`
	NumberOfRooms int       `json:"number_of_rooms"`
	Contacts      []Contact `json:"contacts"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ── Input types ───────────────────────────────────────────────────────────────

// ContactInput carries one contact to be created or replaced.
type ContactInput struct {
	Type  string
	Value string
}

// HotelInput carries all mutable fields for create and update.
type HotelInput struct {
	ChainID    uint
	ManagerID  *uint
	Name       string
	StarRating int
	Street     string
	City       string
	State      string
	Country    string
	Zip        string
	Contacts   []ContactInput
}

// HotelFilter carries optional search parameters for the Search method.
type HotelFilter struct {
	ChainID   *uint
	Name      *string
	City      *string
	State     *string
	Country   *string
	MinStars  *int
	MaxStars  *int
	MinRooms  *int
	MaxRooms  *int
	MinPrice  *float64
	MaxPrice  *float64
	Capacity  *string
	CheckIn   *string // "YYYY-MM-DD"
	CheckOut  *string // "YYYY-MM-DD"
	Limit     int
	Offset    int
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the hotels feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new hotel and its contacts in a single transaction.
func (r *Repository) Create(ctx context.Context, in HotelInput) (*Hotel, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("hotels: begin tx: %w", err)
	}

	const q = `
		INSERT INTO hotels (chain_id, manager_id, name, star_rating, street, city, state, country, zip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := tx.ExecContext(ctx, q,
		in.ChainID, in.ManagerID, in.Name, in.StarRating,
		in.Street, in.City, in.State, in.Country, in.Zip,
	)
	if err != nil {
		tx.Rollback()
		if isForeignKeyViolation(err) {
			return nil, ErrChainNotFound
		}
		return nil, fmt.Errorf("hotels: insert hotel: %w", err)
	}
	id64, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("hotels: last insert id: %w", err)
	}
	hotelID := uint(id64)

	if err := insertContacts(ctx, tx, hotelID, in.Contacts); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("hotels: commit tx: %w", err)
	}

	return r.FindByID(ctx, hotelID)
}

// FindByID retrieves a hotel by primary key. Returns ErrHotelNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Hotel, error) {
	const q = `
		SELECT h.id, h.chain_id, h.manager_id, h.name, h.star_rating,
		       h.street, h.city, h.state, h.country, h.zip,
		       h.created_at, h.updated_at,
		       COUNT(r.id) AS number_of_rooms
		FROM hotels h
		LEFT JOIN rooms r ON r.hotel_id = h.id
		WHERE h.id = ?
		GROUP BY h.id`

	hotel := &Hotel{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&hotel.ID, &hotel.ChainID, &hotel.ManagerID,
		&hotel.Name, &hotel.StarRating,
		&hotel.Street, &hotel.City, &hotel.State, &hotel.Country, &hotel.Zip,
		&hotel.CreatedAt, &hotel.UpdatedAt,
		&hotel.NumberOfRooms,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrHotelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("hotels: find by id: %w", err)
	}

	contacts, err := r.loadContacts(ctx, hotel.ID)
	if err != nil {
		return nil, err
	}
	hotel.Contacts = contacts
	return hotel, nil
}

// Search returns hotels matching the given filter with pagination.
func (r *Repository) Search(ctx context.Context, f HotelFilter) ([]*Hotel, error) {
	var sb strings.Builder
	var args []any
	var having []string

	sb.WriteString(`
		SELECT h.id, h.chain_id, h.manager_id, h.name, h.star_rating,
		       h.street, h.city, h.state, h.country, h.zip,
		       h.created_at, h.updated_at,
		       COUNT(r.id) AS number_of_rooms
		FROM hotels h
		LEFT JOIN rooms r ON r.hotel_id = h.id
		WHERE 1=1`)

	if f.ChainID != nil {
		sb.WriteString(" AND h.chain_id = ?")
		args = append(args, *f.ChainID)
	}
	if f.Name != nil {
		sb.WriteString(" AND h.name LIKE ?")
		args = append(args, "%"+*f.Name+"%")
	}
	if f.City != nil {
		sb.WriteString(" AND h.city = ?")
		args = append(args, *f.City)
	}
	if f.State != nil {
		sb.WriteString(" AND h.state = ?")
		args = append(args, *f.State)
	}
	if f.Country != nil {
		sb.WriteString(" AND h.country = ?")
		args = append(args, *f.Country)
	}
	if f.MinStars != nil {
		sb.WriteString(" AND h.star_rating >= ?")
		args = append(args, *f.MinStars)
	}
	if f.MaxStars != nil {
		sb.WriteString(" AND h.star_rating <= ?")
		args = append(args, *f.MaxStars)
	}
	if f.MinPrice != nil {
		sb.WriteString(" AND EXISTS (SELECT 1 FROM rooms r2 WHERE r2.hotel_id = h.id AND r2.price_per_night >= ?)")
		args = append(args, *f.MinPrice)
	}
	if f.MaxPrice != nil {
		sb.WriteString(" AND EXISTS (SELECT 1 FROM rooms r2 WHERE r2.hotel_id = h.id AND r2.price_per_night <= ?)")
		args = append(args, *f.MaxPrice)
	}
	if f.Capacity != nil {
		sb.WriteString(" AND EXISTS (SELECT 1 FROM rooms r2 WHERE r2.hotel_id = h.id AND r2.capacity = ?)")
		args = append(args, *f.Capacity)
	}
	if f.CheckIn != nil && f.CheckOut != nil {
		// Hotel has at least one available room with no overlapping active booking or renting.
		sb.WriteString(` AND EXISTS (
			SELECT 1 FROM rooms r2
			WHERE r2.hotel_id = h.id
			  AND r2.status = 'available'
			  AND r2.id NOT IN (
			    SELECT b.room_id FROM bookings b
			    WHERE b.status = 'active'
			      AND b.room_id IS NOT NULL
			      AND b.check_in < ? AND b.check_out > ?
			  )
			  AND r2.id NOT IN (
			    SELECT rt.room_id FROM rentings rt
			    WHERE rt.status = 'active'
			      AND rt.room_id IS NOT NULL
			      AND rt.check_in < ? AND rt.check_out > ?
			  )`)
		if f.Capacity != nil {
			sb.WriteString(" AND r2.capacity = ?")
			args = append(args, *f.CheckOut, *f.CheckIn, *f.CheckOut, *f.CheckIn, *f.Capacity)
		} else {
			args = append(args, *f.CheckOut, *f.CheckIn, *f.CheckOut, *f.CheckIn)
		}
		sb.WriteString(")")
	}

	if f.MinRooms != nil {
		having = append(having, "COUNT(r.id) >= ?")
		args = append(args, *f.MinRooms)
	}
	if f.MaxRooms != nil {
		having = append(having, "COUNT(r.id) <= ?")
		args = append(args, *f.MaxRooms)
	}

	sb.WriteString(" GROUP BY h.id")
	if len(having) > 0 {
		sb.WriteString(" HAVING ")
		sb.WriteString(strings.Join(having, " AND "))
	}
	sb.WriteString(" ORDER BY h.id")
	sb.WriteString(" LIMIT ? OFFSET ?")
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("hotels: search: %w", err)
	}
	defer rows.Close()

	var hotels []*Hotel
	var ids []uint
	for rows.Next() {
		h := &Hotel{}
		if err := rows.Scan(
			&h.ID, &h.ChainID, &h.ManagerID,
			&h.Name, &h.StarRating,
			&h.Street, &h.City, &h.State, &h.Country, &h.Zip,
			&h.CreatedAt, &h.UpdatedAt,
			&h.NumberOfRooms,
		); err != nil {
			return nil, fmt.Errorf("hotels: scan hotel: %w", err)
		}
		h.Contacts = []Contact{}
		hotels = append(hotels, h)
		ids = append(ids, h.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hotels: rows: %w", err)
	}

	if len(ids) == 0 {
		return hotels, nil
	}

	contactMap, err := r.loadContactsForIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, h := range hotels {
		if cs, ok := contactMap[h.ID]; ok {
			h.Contacts = cs
		}
	}
	return hotels, nil
}

// Update replaces hotel fields and fully replaces its contacts in a transaction.
func (r *Repository) Update(ctx context.Context, id uint, in HotelInput) (*Hotel, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("hotels: begin tx: %w", err)
	}

	const q = `
		UPDATE hotels
		SET chain_id = ?, manager_id = ?, name = ?, star_rating = ?,
		    street = ?, city = ?, state = ?, country = ?, zip = ?
		WHERE id = ?`
	result, err := tx.ExecContext(ctx, q,
		in.ChainID, in.ManagerID, in.Name, in.StarRating,
		in.Street, in.City, in.State, in.Country, in.Zip, id,
	)
	if err != nil {
		tx.Rollback()
		if isForeignKeyViolation(err) {
			return nil, ErrChainNotFound
		}
		return nil, fmt.Errorf("hotels: update hotel: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("hotels: rows affected: %w", err)
	}
	if n == 0 {
		tx.Rollback()
		return nil, ErrHotelNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM hotel_contacts WHERE hotel_id = ?`, id); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("hotels: delete contacts: %w", err)
	}

	if err := insertContacts(ctx, tx, id, in.Contacts); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("hotels: commit tx: %w", err)
	}

	return r.FindByID(ctx, id)
}

// Delete removes a hotel by primary key. Cascades to rooms via FK.
func (r *Repository) Delete(ctx context.Context, id uint) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM hotels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("hotels: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("hotels: rows affected: %w", err)
	}
	if n == 0 {
		return ErrHotelNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func insertContacts(ctx context.Context, tx *sql.Tx, hotelID uint, contacts []ContactInput) error {
	if len(contacts) == 0 {
		return nil
	}
	const q = `INSERT INTO hotel_contacts (hotel_id, type, value) VALUES (?, ?, ?)`
	for _, c := range contacts {
		if _, err := tx.ExecContext(ctx, q, hotelID, c.Type, c.Value); err != nil {
			return fmt.Errorf("hotels: insert contact: %w", err)
		}
	}
	return nil
}

func (r *Repository) loadContacts(ctx context.Context, hotelID uint) ([]Contact, error) {
	const q = `SELECT id, type, value FROM hotel_contacts WHERE hotel_id = ? ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q, hotelID)
	if err != nil {
		return nil, fmt.Errorf("hotels: load contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Type, &c.Value); err != nil {
			return nil, fmt.Errorf("hotels: scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}
	if contacts == nil {
		contacts = []Contact{}
	}
	return contacts, rows.Err()
}

func (r *Repository) loadContactsForIDs(ctx context.Context, ids []uint) (map[uint][]Contact, error) {
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	q := fmt.Sprintf(`SELECT id, hotel_id, type, value FROM hotel_contacts WHERE hotel_id IN (%s) ORDER BY id`, placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("hotels: load contacts for ids: %w", err)
	}
	defer rows.Close()

	m := make(map[uint][]Contact)
	for rows.Next() {
		var c Contact
		var hotelID uint
		if err := rows.Scan(&c.ID, &hotelID, &c.Type, &c.Value); err != nil {
			return nil, fmt.Errorf("hotels: scan contact: %w", err)
		}
		m[hotelID] = append(m[hotelID], c)
	}
	return m, rows.Err()
}

// isForeignKeyViolation reports whether err is a MySQL FK constraint failure (1452).
func isForeignKeyViolation(err error) bool {
	return strings.Contains(err.Error(), "a foreign key constraint fails")
}
