package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "unique violation of target constraint",
			err:  &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: usersLoginKey},
			want: true,
		},
		{
			name: "wrapped unique violation",
			err:  fmt.Errorf("insert: %w", &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: usersLoginKey}),
			want: true,
		},
		{
			name: "unique violation of another constraint",
			err:  &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "users_email_key"},
		},
		{
			name: "other postgres error",
			err:  &pgconn.PgError{Code: pgerrcode.NotNullViolation, ConstraintName: usersLoginKey},
		},
		{name: "non-postgres error", err: errors.New("connection refused")},
		{name: "nil error", err: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUniqueViolation(tt.err, usersLoginKey))
		})
	}
}
