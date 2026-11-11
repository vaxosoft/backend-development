package shortener

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// All queries are parameterised and every *sql.Rows is closed with rows.Err() checked.

// CreateUser inserts a user and returns it with its new id.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES (?, ?)`, email, passwordHash)
	if isUnique(err) {
		return User{}, ErrConflict
	}
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("user id: %w", err)
	}
	return User{ID: id, Email: email, PasswordHash: passwordHash}, nil
}

// UserByEmail returns the user with that e-mail, or ErrNotFound.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	u := User{Email: email}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, password_hash FROM users WHERE email = ?`, email).Scan(&u.ID, &u.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user: %w", err)
	}
	return u, nil
}

// CreateLink stores a link for the owner.
func (s *Store) CreateLink(ctx context.Context, ownerID int64, slug, url string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO links (slug, url, owner_id) VALUES (?, ?, ?)`, slug, url, ownerID)
	if isUnique(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}
	return nil
}

// LinkBySlug returns the link, or ErrNotFound.
func (s *Store) LinkBySlug(ctx context.Context, slug string) (Link, error) {
	l := Link{Slug: slug}
	err := s.db.QueryRowContext(ctx,
		`SELECT url, hits, owner_id FROM links WHERE slug = ?`, slug).Scan(&l.URL, &l.Hits, &l.OwnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrNotFound
	}
	if err != nil {
		return Link{}, fmt.Errorf("select link: %w", err)
	}
	return l, nil
}

// Hit increments the hit counter and returns the target URL, or ErrNotFound.
// One UPDATE ... RETURNING statement is atomic, so concurrent hits never lose a count.
func (s *Store) Hit(ctx context.Context, slug string) (string, error) {
	var url string
	err := s.db.QueryRowContext(ctx,
		`UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url`, slug).Scan(&url)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("count hit: %w", err)
	}
	return url, nil
}

// LinksByOwner returns one page of the owner's links (newest first) and the
// total number of links the owner has.
func (s *Store) LinksByOwner(ctx context.Context, ownerID int64, limit, offset int) ([]Link, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM links WHERE owner_id = ?`, ownerID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count links: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, url, hits FROM links WHERE owner_id = ? ORDER BY id DESC LIMIT ? OFFSET ?`,
		ownerID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("select links: %w", err)
	}
	defer rows.Close()

	links := []Link{}
	for rows.Next() {
		l := Link{OwnerID: ownerID}
		if err := rows.Scan(&l.Slug, &l.URL, &l.Hits); err != nil {
			return nil, 0, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate links: %w", err)
	}
	return links, total, nil
}

// DeleteLink removes the link, or returns ErrNotFound.
func (s *Store) DeleteLink(ctx context.Context, slug string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM links WHERE slug = ?`, slug)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}
