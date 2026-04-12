package employees

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
	ErrEmployeeNotFound = errors.New("employees: employee not found")
	ErrSSNTaken         = errors.New("employees: SSN already in use")
	ErrHotelNotFound    = errors.New("employees: hotel not found")
)

// ── Domain types ──────────────────────────────────────────────────────────────

// Position mirrors one row in employee_positions.
type Position struct {
	ID       uint   `json:"id"`
	HotelID  uint   `json:"hotel_id"`
	Position string `json:"position"`
}

// ManagedHotel is a slim hotel summary returned inside the employee response.
type ManagedHotel struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// Employee mirrors the employees table plus derived slices.
type Employee struct {
	ID            uint           `json:"id"`
	UserID        *uint          `json:"user_id"`
	FirstName     string         `json:"first_name"`
	LastName      string         `json:"last_name"`
	Street        string         `json:"street"`
	City          string         `json:"city"`
	State         string         `json:"state"`
	Country       string         `json:"country"`
	Zip           string         `json:"zip"`
	SSN           string         `json:"ssn"`
	Positions     []Position     `json:"positions"`
	ManagedHotels []ManagedHotel `json:"managed_hotels"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// ── Input types ───────────────────────────────────────────────────────────────

// PositionInput carries one position to be created or replaced.
type PositionInput struct {
	HotelID  uint
	Position string
}

// EmployeeInput carries all mutable fields for update.
type EmployeeInput struct {
	FirstName string
	LastName  string
	Street    string
	City      string
	State     string
	Country   string
	Zip       string
	SSN       string
	Positions []PositionInput
}

// EmployeeFilter carries optional search parameters for the FindAll method.
type EmployeeFilter struct {
	HotelID *uint   // filter via employee_positions JOIN
	Name    *string // LIKE on CONCAT(first_name, ' ', last_name)
	Limit   int
	Offset  int
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the employees feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// FindByUserID retrieves an employee by their linked user_id. Returns ErrEmployeeNotFound when absent.
func (r *Repository) FindByUserID(ctx context.Context, userID uint) (*Employee, error) {
	const q = `
		SELECT id, user_id, first_name, last_name,
		       street, city, state, country, zip, ssn,
		       created_at, updated_at
		FROM employees
		WHERE user_id = ?
		LIMIT 1`

	emp := &Employee{}
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&emp.ID, &emp.UserID,
		&emp.FirstName, &emp.LastName,
		&emp.Street, &emp.City, &emp.State, &emp.Country, &emp.Zip,
		&emp.SSN, &emp.CreatedAt, &emp.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmployeeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("employees: find by user id: %w", err)
	}

	positions, err := r.loadPositions(ctx, emp.ID)
	if err != nil {
		return nil, err
	}
	emp.Positions = positions

	managed, err := r.loadManagedHotels(ctx, emp.ID)
	if err != nil {
		return nil, err
	}
	emp.ManagedHotels = managed

	return emp, nil
}

// ReplacePositions swaps all positions for the given employee in a transaction.
func (r *Repository) ReplacePositions(ctx context.Context, employeeID uint, positions []PositionInput) (*Employee, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("employees: begin tx: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM employee_positions WHERE employee_id = ?`, employeeID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("employees: delete positions: %w", err)
	}

	if err := insertPositions(ctx, tx, employeeID, positions); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("employees: commit tx: %w", err)
	}

	return r.FindByID(ctx, employeeID)
}

// FindByID retrieves an employee by primary key. Returns ErrEmployeeNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Employee, error) {
	const q = `
		SELECT id, user_id, first_name, last_name,
		       street, city, state, country, zip, ssn,
		       created_at, updated_at
		FROM employees
		WHERE id = ?
		LIMIT 1`

	emp := &Employee{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&emp.ID, &emp.UserID,
		&emp.FirstName, &emp.LastName,
		&emp.Street, &emp.City, &emp.State, &emp.Country, &emp.Zip,
		&emp.SSN, &emp.CreatedAt, &emp.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEmployeeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("employees: find by id: %w", err)
	}

	positions, err := r.loadPositions(ctx, emp.ID)
	if err != nil {
		return nil, err
	}
	emp.Positions = positions

	managed, err := r.loadManagedHotels(ctx, emp.ID)
	if err != nil {
		return nil, err
	}
	emp.ManagedHotels = managed

	return emp, nil
}

// FindAll returns employees matching the given filter with pagination.
func (r *Repository) FindAll(ctx context.Context, f EmployeeFilter) ([]*Employee, error) {
	var sb strings.Builder
	var args []any

	sb.WriteString(`
		SELECT DISTINCT e.id, e.user_id, e.first_name, e.last_name,
		       e.street, e.city, e.state, e.country, e.zip, e.ssn,
		       e.created_at, e.updated_at
		FROM employees e`)

	if f.HotelID != nil {
		sb.WriteString(" INNER JOIN employee_positions ep ON ep.employee_id = e.id AND ep.hotel_id = ?")
		args = append(args, *f.HotelID)
	}

	sb.WriteString(" WHERE 1=1")

	if f.Name != nil {
		sb.WriteString(" AND CONCAT(e.first_name, ' ', e.last_name) LIKE ?")
		args = append(args, "%"+*f.Name+"%")
	}

	sb.WriteString(" ORDER BY e.id LIMIT ? OFFSET ?")
	args = append(args, f.Limit, f.Offset)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("employees: find all: %w", err)
	}
	defer rows.Close()

	var employees []*Employee
	var ids []uint
	for rows.Next() {
		emp := &Employee{}
		if err := rows.Scan(
			&emp.ID, &emp.UserID,
			&emp.FirstName, &emp.LastName,
			&emp.Street, &emp.City, &emp.State, &emp.Country, &emp.Zip,
			&emp.SSN, &emp.CreatedAt, &emp.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("employees: scan employee: %w", err)
		}
		emp.Positions = []Position{}
		emp.ManagedHotels = []ManagedHotel{}
		employees = append(employees, emp)
		ids = append(ids, emp.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("employees: rows: %w", err)
	}

	if len(ids) == 0 {
		return employees, nil
	}

	posMap, err := r.loadPositionsForIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	managedMap, err := r.loadManagedHotelsForIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, emp := range employees {
		if ps, ok := posMap[emp.ID]; ok {
			emp.Positions = ps
		}
		if mh, ok := managedMap[emp.ID]; ok {
			emp.ManagedHotels = mh
		}
	}

	return employees, nil
}

// Update replaces employee fields and fully replaces positions in a transaction.
func (r *Repository) Update(ctx context.Context, id uint, in EmployeeInput) (*Employee, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("employees: begin tx: %w", err)
	}

	const q = `
		UPDATE employees
		SET first_name = ?, last_name = ?,
		    street = ?, city = ?, state = ?, country = ?, zip = ?,
		    ssn = ?
		WHERE id = ?`
	result, err := tx.ExecContext(ctx, q,
		in.FirstName, in.LastName,
		in.Street, in.City, in.State, in.Country, in.Zip,
		in.SSN, id,
	)
	if err != nil {
		tx.Rollback()
		if isDuplicateEntry(err) {
			return nil, ErrSSNTaken
		}
		return nil, fmt.Errorf("employees: update employee: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("employees: rows affected: %w", err)
	}
	if n == 0 {
		tx.Rollback()
		return nil, ErrEmployeeNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM employee_positions WHERE employee_id = ?`, id); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("employees: delete positions: %w", err)
	}

	if err := insertPositions(ctx, tx, id, in.Positions); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("employees: commit tx: %w", err)
	}

	return r.FindByID(ctx, id)
}

// Delete removes an employee by primary key.
func (r *Repository) Delete(ctx context.Context, id uint) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM employees WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("employees: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("employees: rows affected: %w", err)
	}
	if n == 0 {
		return ErrEmployeeNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func insertPositions(ctx context.Context, tx *sql.Tx, employeeID uint, positions []PositionInput) error {
	if len(positions) == 0 {
		return nil
	}
	const q = `INSERT INTO employee_positions (employee_id, hotel_id, position) VALUES (?, ?, ?)`
	for _, p := range positions {
		if _, err := tx.ExecContext(ctx, q, employeeID, p.HotelID, p.Position); err != nil {
			if isForeignKeyViolation(err) {
				return ErrHotelNotFound
			}
			return fmt.Errorf("employees: insert position: %w", err)
		}
	}
	return nil
}

func (r *Repository) loadPositions(ctx context.Context, employeeID uint) ([]Position, error) {
	const q = `SELECT id, hotel_id, position FROM employee_positions WHERE employee_id = ? ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q, employeeID)
	if err != nil {
		return nil, fmt.Errorf("employees: load positions: %w", err)
	}
	defer rows.Close()

	var positions []Position
	for rows.Next() {
		var p Position
		if err := rows.Scan(&p.ID, &p.HotelID, &p.Position); err != nil {
			return nil, fmt.Errorf("employees: scan position: %w", err)
		}
		positions = append(positions, p)
	}
	if positions == nil {
		positions = []Position{}
	}
	return positions, rows.Err()
}

func (r *Repository) loadPositionsForIDs(ctx context.Context, ids []uint) (map[uint][]Position, error) {
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	q := fmt.Sprintf(`SELECT id, employee_id, hotel_id, position FROM employee_positions WHERE employee_id IN (%s) ORDER BY id`, placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("employees: load positions for ids: %w", err)
	}
	defer rows.Close()

	m := make(map[uint][]Position)
	for rows.Next() {
		var p Position
		var empID uint
		if err := rows.Scan(&p.ID, &empID, &p.HotelID, &p.Position); err != nil {
			return nil, fmt.Errorf("employees: scan position: %w", err)
		}
		m[empID] = append(m[empID], p)
	}
	return m, rows.Err()
}

func (r *Repository) loadManagedHotels(ctx context.Context, employeeID uint) ([]ManagedHotel, error) {
	const q = `SELECT id, name FROM hotels WHERE manager_id = ? ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q, employeeID)
	if err != nil {
		return nil, fmt.Errorf("employees: load managed hotels: %w", err)
	}
	defer rows.Close()

	var hotels []ManagedHotel
	for rows.Next() {
		var h ManagedHotel
		if err := rows.Scan(&h.ID, &h.Name); err != nil {
			return nil, fmt.Errorf("employees: scan managed hotel: %w", err)
		}
		hotels = append(hotels, h)
	}
	if hotels == nil {
		hotels = []ManagedHotel{}
	}
	return hotels, rows.Err()
}

func (r *Repository) loadManagedHotelsForIDs(ctx context.Context, ids []uint) (map[uint][]ManagedHotel, error) {
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	q := fmt.Sprintf(`SELECT manager_id, id, name FROM hotels WHERE manager_id IN (%s) ORDER BY id`, placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("employees: load managed hotels for ids: %w", err)
	}
	defer rows.Close()

	m := make(map[uint][]ManagedHotel)
	for rows.Next() {
		var h ManagedHotel
		var managerID uint
		if err := rows.Scan(&managerID, &h.ID, &h.Name); err != nil {
			return nil, fmt.Errorf("employees: scan managed hotel: %w", err)
		}
		m[managerID] = append(m[managerID], h)
	}
	return m, rows.Err()
}

func isDuplicateEntry(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry")
}

func isForeignKeyViolation(err error) bool {
	return strings.Contains(err.Error(), "a foreign key constraint fails")
}
