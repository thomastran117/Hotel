package reviews

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
	ErrReviewNotFound  = errors.New("reviews: review not found")
	ErrHotelNotFound   = errors.New("reviews: hotel not found")
	ErrAlreadyReviewed = errors.New("reviews: user has already reviewed this hotel")
	ErrNotOwner        = errors.New("reviews: user is not the owner of this review")
)

// ── Domain types ──────────────────────────────────────────────────────────────

// Review mirrors one row in the reviews table.
type Review struct {
	ID        uint      `json:"id"`
	HotelID   *uint     `json:"hotel_id"`
	UserID    *uint     `json:"user_id"`
	Rating    uint8     `json:"rating"`
	Comment   *string   `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// ── Input types ───────────────────────────────────────────────────────────────

// ReviewInput carries all fields required to create a review.
type ReviewInput struct {
	HotelID uint
	UserID  uint
	Rating  uint8
	Comment *string
}

// ── Repository ────────────────────────────────────────────────────────────────

// Repository handles all SQL operations for the reviews feature.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a Repository backed by the given connection pool.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new review and returns the persisted row.
func (r *Repository) Create(ctx context.Context, in ReviewInput) (*Review, error) {
	const q = `
		INSERT INTO reviews (hotel_id, user_id, rating, comment)
		VALUES (?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, q, in.HotelID, in.UserID, in.Rating, in.Comment)
	if err != nil {
		if isForeignKeyViolation(err) {
			return nil, ErrHotelNotFound
		}
		if isDuplicateEntry(err) {
			return nil, ErrAlreadyReviewed
		}
		return nil, fmt.Errorf("reviews: insert: %w", err)
	}
	id64, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("reviews: last insert id: %w", err)
	}
	return r.FindByID(ctx, uint(id64))
}

// FindByID retrieves a review by primary key. Returns ErrReviewNotFound when absent.
func (r *Repository) FindByID(ctx context.Context, id uint) (*Review, error) {
	const q = `
		SELECT id, hotel_id, user_id, rating, comment, created_at
		FROM reviews WHERE id = ?`
	rev := &Review{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&rev.ID, &rev.HotelID, &rev.UserID, &rev.Rating, &rev.Comment, &rev.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrReviewNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reviews: find by id: %w", err)
	}
	return rev, nil
}

// ListByHotel returns reviews for a hotel ordered by most recent first.
func (r *Repository) ListByHotel(ctx context.Context, hotelID uint, limit, offset int) ([]*Review, error) {
	const q = `
		SELECT id, hotel_id, user_id, rating, comment, created_at
		FROM reviews
		WHERE hotel_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`
	rows, err := r.db.QueryContext(ctx, q, hotelID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("reviews: list by hotel: %w", err)
	}
	defer rows.Close()

	var reviews []*Review
	for rows.Next() {
		rev := &Review{}
		if err := rows.Scan(&rev.ID, &rev.HotelID, &rev.UserID, &rev.Rating, &rev.Comment, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("reviews: scan: %w", err)
		}
		reviews = append(reviews, rev)
	}
	if reviews == nil {
		reviews = []*Review{}
	}
	return reviews, rows.Err()
}

// Delete removes a review by primary key. Returns ErrReviewNotFound when absent.
func (r *Repository) Delete(ctx context.Context, id uint) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM reviews WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("reviews: delete: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("reviews: rows affected: %w", err)
	}
	if n == 0 {
		return ErrReviewNotFound
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func isForeignKeyViolation(err error) bool {
	return strings.Contains(err.Error(), "a foreign key constraint fails")
}

func isDuplicateEntry(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry")
}
