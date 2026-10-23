package learning

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func isUniqueViolation(err error, constraint string) bool {
	return isConstraintViolation(err, "23505", constraint)
}

func isForeignKeyViolation(err error, constraint string) bool {
	return isConstraintViolation(err, "23503", constraint)
}

func isConstraintViolation(err error, code, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code && pgErr.ConstraintName == constraint
}
