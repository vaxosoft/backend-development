// Package deps pins third-party dependencies for the whole repo. Not a lesson.
package deps

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "golang.org/x/crypto/bcrypt"
	_ "golang.org/x/sync/errgroup"
	_ "golang.org/x/time/rate"
	_ "gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)
