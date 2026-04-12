package customers

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
	ErrCustomerNotFound = errors.New("customers: customer not found")
	ErrIdentityTaken    = errors.New("customers: identity document already in use")
)

// ── Domain types ──────────────────────────────────────────────────────────────

// Customer mirrors the customers table.
type Customer struct {
	ID           uint      `json:"id"`
	UserID       *uint     `json:"user_id"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Street       string    `json:"street"`
	City         string    `json:"city"`
	State        string    `json:"state"`
	Country      string    `json:"country"`
	Zip          string    `json:"zip"`
	IDType       string    `json:"id_type"`
	IDNumber     string    `json:"id_number"`
	RegisteredAt time.Time `json:"registered_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ── Input types ───────────────────────────────────────────────────────────────

// CustomerInput carries all mutable fields for update.
type CustomerInput struct {
	FirstName string
	LastName  string
	Street    string
	City      string
	State     string
	Country   string
	Zip       string
	IDType    string
	IDNumber  string
}

// CustomerFilter carries optional search parameters for the FindAll method.
type CustomerFilter struct {
	Name   *string // LIKE on CONCAT(first_name, ' ', last_name)
	Limit  int
	Offset int
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the customers feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// FindByID retrieves a customer by primary key. Returns ErrCustomerNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Customer, error) {
	const q = `
		SELECT id, user_id, first_name, last_name,
		       street, city, state, country, zip,
		       id_type, id_number, registered_at,
		       created_at, updated_at
		FROM customers
		WHERE id = ?
		LIMIT 1`

	c := &Customer{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID, &c.UserID,
		&c.FirstName, &c.LastName,
		&c.Street, &c.City, &c.State, &c.Country, &c.Zip,
		&c.IDType, &c.IDNumber, &c.RegisteredAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCustomerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("customers: find by id: %w", err)
	}
	return c, nil
}

// FindAll returns customers matching the given filter with pagination.
func (r *Repository) FindAll(ctx context.Context, f CustomerFilter) ([]*Customer, error) {
	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT id, user_id, first_name, last_name,
		       street, city, state, country, zip,
		       id_type, id_number, registered_at,
		       created_at, updated_at
		FROM customers
		WHERE 1=1`)

	if f.Name != nil {
		sb.WriteString(" AND CONCAT(first_name, ' ', last_name) LIKE ?")
		args = append(args, "%"+*f.Name+"%")
	}

	sb.WriteString(" ORDER BY id LIMIT ? OFFSET ?")
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("customers: find all: %w", err)
	}
	defer rows.Close()

	var customers []*Customer
	for rows.Next() {
		c := &Customer{}
		if err := rows.Scan(
			&c.ID, &c.UserID,
			&c.FirstName, &c.LastName,
			&c.Street, &c.City, &c.State, &c.Country, &c.Zip,
			&c.IDType, &c.IDNumber, &c.RegisteredAt,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("customers: scan customer: %w", err)
		}
		customers = append(customers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("customers: rows: %w", err)
	}

	return customers, nil
}

// Update replaces customer fields.
func (r *Repository) Update(ctx context.Context, id uint, in CustomerInput) (*Customer, error) {
	const q = `
		UPDATE customers
		SET first_name = ?, last_name = ?,
		    street = ?, city = ?, state = ?, country = ?, zip = ?,
		    id_type = ?, id_number = ?
		WHERE id = ?`
	result, err := r.db.ExecContext(ctx, q,
		in.FirstName, in.LastName,
		in.Street, in.City, in.State, in.Country, in.Zip,
		in.IDType, in.IDNumber, id,
	)
	if err != nil {
		if isDuplicateEntry(err) {
			return nil, ErrIdentityTaken
		}
		return nil, fmt.Errorf("customers: update customer: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("customers: rows affected: %w", err)
	}
	if n == 0 {
		return nil, ErrCustomerNotFound
	}
	return r.FindByID(ctx, id)
}

// Delete removes a customer by primary key.
func (r *Repository) Delete(ctx context.Context, id uint) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM customers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("customers: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("customers: rows affected: %w", err)
	}
	if n == 0 {
		return ErrCustomerNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func isDuplicateEntry(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry")
}
