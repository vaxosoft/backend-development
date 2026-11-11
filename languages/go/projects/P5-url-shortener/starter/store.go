package shortener

import (
	"context"
	"errors"
)

var errTODO = errors.New("TODO: not implemented")

// All queries must be parameterised (placeholders, never string concatenation)
// and every *sql.Rows must be closed, with rows.Err() checked.

// CreateUser inserts a user and returns it with its new id.
// TODO: return ErrConflict (use isUnique) when the e-mail is already taken.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	return User{}, errTODO
}

// UserByEmail returns the user with that e-mail, or ErrNotFound.
// TODO: implement with QueryRowContext and map sql.ErrNoRows to ErrNotFound.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return User{}, errTODO
}

// CreateLink stores a link for the owner.
// TODO: return ErrConflict when the slug already exists.
func (s *Store) CreateLink(ctx context.Context, ownerID int64, slug, url string) error {
	return errTODO
}

// LinkBySlug returns the link, or ErrNotFound.
func (s *Store) LinkBySlug(ctx context.Context, slug string) (Link, error) {
	return Link{}, errTODO
}

// Hit increments the hit counter and returns the target URL, or ErrNotFound.
// TODO: do it in ONE statement (UPDATE ... RETURNING url) so concurrent
// requests can never lose a count.
func (s *Store) Hit(ctx context.Context, slug string) (string, error) {
	return "", errTODO
}

// LinksByOwner returns one page of the owner's links (newest first) and the
// total number of links the owner has.
func (s *Store) LinksByOwner(ctx context.Context, ownerID int64, limit, offset int) ([]Link, int, error) {
	return nil, 0, errTODO
}

// DeleteLink removes the link, or returns ErrNotFound.
func (s *Store) DeleteLink(ctx context.Context, slug string) error {
	return errTODO
}
