package auth

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
	ErrUserNotFound    = errors.New("auth: user not found")
	ErrEmailTaken      = errors.New("auth: email already in use")
	ErrIdentityTaken   = errors.New("auth: identity document already registered")
	ErrSSNTaken        = errors.New("auth: SSN already registered")
)

// ── Domain types ──────────────────────────────────────────────────────────────

// User mirrors the users table.
type User struct {
	ID           uint
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CustomerProfile holds the fields written to the customers table on signup.
type CustomerProfile struct {
	FirstName string
	LastName  string
	Street    string
	City      string
	State     string
	Country   string
	Zip       string
	IDType    string // SSN | SIN | driving_licence
	IDNumber  string
}

// EmployeeProfile holds the fields written to the employees table on signup.
type EmployeeProfile struct {
	FirstName string
	LastName  string
	Street    string
	City      string
	State     string
	Country   string
	Zip       string
	SSN       string
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the auth feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// FindByEmail retrieves a user by email. Returns ErrUserNotFound when absent.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
		SELECT id, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE email = ?
		LIMIT 1`

	u := &User{}
	err := r.db.QueryRowContext(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: find user by email: %w", err)
	}
	return u, nil
}

// FindByID retrieves a user by primary key. Returns ErrUserNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*User, error) {
	const q = `
		SELECT id, email, password_hash, role, created_at, updated_at
		FROM users
		WHERE id = ?
		LIMIT 1`

	u := &User{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: find user by id: %w", err)
	}
	return u, nil
}

// CreateWithCustomer inserts a users row and a customers row in a single
// transaction. Returns ErrEmailTaken or ErrIdentityTaken on duplicate keys.
func (r *Repository) CreateWithCustomer(ctx context.Context, email, passwordHash string, p CustomerProfile) (*User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: begin tx: %w", err)
	}

	userID, err := insertUser(ctx, tx, email, passwordHash, "guest")
	if err != nil {
		tx.Rollback()
		if isDuplicateEntry(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("auth: insert user: %w", err)
	}

	const q = `
		INSERT INTO customers
			(user_id, first_name, last_name, street, city, state, country, zip, id_type, id_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, q,
		userID, p.FirstName, p.LastName,
		p.Street, p.City, p.State, p.Country, p.Zip,
		p.IDType, p.IDNumber,
	)
	if err != nil {
		tx.Rollback()
		if isDuplicateEntry(err) {
			return nil, ErrIdentityTaken
		}
		return nil, fmt.Errorf("auth: insert customer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("auth: commit tx: %w", err)
	}

	return r.FindByID(ctx, userID)
}

// CreateWithEmployee inserts a users row and an employees row in a single
// transaction. role must be "staff" or "admin".
// Returns ErrEmailTaken or ErrSSNTaken on duplicate keys.
func (r *Repository) CreateWithEmployee(ctx context.Context, email, passwordHash, role string, p EmployeeProfile) (*User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: begin tx: %w", err)
	}

	userID, err := insertUser(ctx, tx, email, passwordHash, role)
	if err != nil {
		tx.Rollback()
		if isDuplicateEntry(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("auth: insert user: %w", err)
	}

	const q = `
		INSERT INTO employees
			(user_id, first_name, last_name, street, city, state, country, zip, ssn)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.ExecContext(ctx, q,
		userID, p.FirstName, p.LastName,
		p.Street, p.City, p.State, p.Country, p.Zip,
		p.SSN,
	)
	if err != nil {
		tx.Rollback()
		if isDuplicateEntry(err) {
			return nil, ErrSSNTaken
		}
		return nil, fmt.Errorf("auth: insert employee: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("auth: commit tx: %w", err)
	}

	return r.FindByID(ctx, userID)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// insertUser executes the users INSERT within an existing transaction and
// returns the new row's ID.
func insertUser(ctx context.Context, tx *sql.Tx, email, passwordHash, role string) (uint, error) {
	const q = `INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)`
	result, err := tx.ExecContext(ctx, q, email, passwordHash, role)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

// isDuplicateEntry reports whether err is a MySQL duplicate-entry error (1062).
// We avoid importing the driver package by matching the error message.
func isDuplicateEntry(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry")
}
