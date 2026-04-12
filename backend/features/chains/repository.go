package chains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var ErrChainNotFound = errors.New("chains: chain not found")

// ── Domain types ──────────────────────────────────────────────────────────────

// Contact represents one entry in the chain_contacts table.
type Contact struct {
	ID    uint   `json:"id"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Chain mirrors the chains table with derived fields.
type Chain struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	OfficeStreet   string    `json:"office_street"`
	OfficeCity     string    `json:"office_city"`
	OfficeState    string    `json:"office_state"`
	OfficeCountry  string    `json:"office_country"`
	OfficeZip      string    `json:"office_zip"`
	NumberOfHotels int       `json:"number_of_hotels"`
	Contacts       []Contact `json:"contacts"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ── Input types ───────────────────────────────────────────────────────────────

// ContactInput carries one contact to be created or replaced.
type ContactInput struct {
	Type  string
	Value string
}

// ChainInput carries all mutable fields for create and update.
type ChainInput struct {
	Name          string
	OfficeStreet  string
	OfficeCity    string
	OfficeState   string
	OfficeCountry string
	OfficeZip     string
	Contacts      []ContactInput
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the chains feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new chain and its contacts in a single transaction.
func (r *Repository) Create(ctx context.Context, in ChainInput) (*Chain, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("chains: begin tx: %w", err)
	}

	const q = `
		INSERT INTO chains (name, office_street, office_city, office_state, office_country, office_zip)
		VALUES (?, ?, ?, ?, ?, ?)`
	result, err := tx.ExecContext(ctx, q,
		in.Name, in.OfficeStreet, in.OfficeCity, in.OfficeState, in.OfficeCountry, in.OfficeZip,
	)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("chains: insert chain: %w", err)
	}
	id64, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("chains: last insert id: %w", err)
	}
	chainID := uint(id64)

	if err := insertContacts(ctx, tx, chainID, in.Contacts); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("chains: commit tx: %w", err)
	}

	return r.FindByID(ctx, chainID)
}

// FindByID retrieves a chain by primary key. Returns ErrChainNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Chain, error) {
	const q = `
		SELECT c.id, c.name, c.office_street, c.office_city, c.office_state,
		       c.office_country, c.office_zip, c.created_at, c.updated_at,
		       COUNT(h.id) AS number_of_hotels
		FROM chains c
		LEFT JOIN hotels h ON h.chain_id = c.id
		WHERE c.id = ?
		GROUP BY c.id`

	chain := &Chain{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&chain.ID, &chain.Name,
		&chain.OfficeStreet, &chain.OfficeCity, &chain.OfficeState,
		&chain.OfficeCountry, &chain.OfficeZip,
		&chain.CreatedAt, &chain.UpdatedAt,
		&chain.NumberOfHotels,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChainNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("chains: find by id: %w", err)
	}

	contacts, err := r.loadContacts(ctx, chain.ID)
	if err != nil {
		return nil, err
	}
	chain.Contacts = contacts
	return chain, nil
}

// List returns all chains with derived hotel counts and their contacts.
func (r *Repository) List(ctx context.Context) ([]*Chain, error) {
	const q = `
		SELECT c.id, c.name, c.office_street, c.office_city, c.office_state,
		       c.office_country, c.office_zip, c.created_at, c.updated_at,
		       COUNT(h.id) AS number_of_hotels
		FROM chains c
		LEFT JOIN hotels h ON h.chain_id = c.id
		GROUP BY c.id
		ORDER BY c.id`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("chains: list: %w", err)
	}
	defer rows.Close()

	var chains []*Chain
	var ids []uint
	for rows.Next() {
		ch := &Chain{}
		if err := rows.Scan(
			&ch.ID, &ch.Name,
			&ch.OfficeStreet, &ch.OfficeCity, &ch.OfficeState,
			&ch.OfficeCountry, &ch.OfficeZip,
			&ch.CreatedAt, &ch.UpdatedAt,
			&ch.NumberOfHotels,
		); err != nil {
			return nil, fmt.Errorf("chains: scan chain: %w", err)
		}
		ch.Contacts = []Contact{}
		chains = append(chains, ch)
		ids = append(ids, ch.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chains: rows: %w", err)
	}

	if len(ids) == 0 {
		return chains, nil
	}

	contactMap, err := r.loadContactsForIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, ch := range chains {
		if cs, ok := contactMap[ch.ID]; ok {
			ch.Contacts = cs
		}
	}
	return chains, nil
}

// Update replaces chain fields and fully replaces its contacts in a transaction.
func (r *Repository) Update(ctx context.Context, id uint, in ChainInput) (*Chain, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("chains: begin tx: %w", err)
	}

	const q = `
		UPDATE chains
		SET name = ?, office_street = ?, office_city = ?, office_state = ?,
		    office_country = ?, office_zip = ?
		WHERE id = ?`
	result, err := tx.ExecContext(ctx, q,
		in.Name, in.OfficeStreet, in.OfficeCity, in.OfficeState,
		in.OfficeCountry, in.OfficeZip, id,
	)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("chains: update chain: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("chains: rows affected: %w", err)
	}
	if n == 0 {
		tx.Rollback()
		return nil, ErrChainNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM chain_contacts WHERE chain_id = ?`, id); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("chains: delete contacts: %w", err)
	}

	if err := insertContacts(ctx, tx, id, in.Contacts); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("chains: commit tx: %w", err)
	}

	return r.FindByID(ctx, id)
}

// Delete removes a chain by primary key. Cascades to hotels and rooms via FK.
func (r *Repository) Delete(ctx context.Context, id uint) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM chains WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("chains: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("chains: rows affected: %w", err)
	}
	if n == 0 {
		return ErrChainNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func insertContacts(ctx context.Context, tx *sql.Tx, chainID uint, contacts []ContactInput) error {
	if len(contacts) == 0 {
		return nil
	}
	const q = `INSERT INTO chain_contacts (chain_id, type, value) VALUES (?, ?, ?)`
	for _, c := range contacts {
		if _, err := tx.ExecContext(ctx, q, chainID, c.Type, c.Value); err != nil {
			return fmt.Errorf("chains: insert contact: %w", err)
		}
	}
	return nil
}

func (r *Repository) loadContacts(ctx context.Context, chainID uint) ([]Contact, error) {
	const q = `SELECT id, type, value FROM chain_contacts WHERE chain_id = ? ORDER BY id`
	rows, err := r.db.QueryContext(ctx, q, chainID)
	if err != nil {
		return nil, fmt.Errorf("chains: load contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Type, &c.Value); err != nil {
			return nil, fmt.Errorf("chains: scan contact: %w", err)
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

	q := fmt.Sprintf(`SELECT id, chain_id, type, value FROM chain_contacts WHERE chain_id IN (%s) ORDER BY id`, placeholders)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("chains: load contacts for ids: %w", err)
	}
	defer rows.Close()

	m := make(map[uint][]Contact)
	for rows.Next() {
		var c Contact
		var chainID uint
		if err := rows.Scan(&c.ID, &chainID, &c.Type, &c.Value); err != nil {
			return nil, fmt.Errorf("chains: scan contact: %w", err)
		}
		m[chainID] = append(m[chainID], c)
	}
	return m, rows.Err()
}
