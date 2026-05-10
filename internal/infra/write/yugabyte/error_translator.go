package repository

import (
	"database/sql"
	"errors"
	"strings"

	"gig-service/internal/domain"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
)

const (
	GigsPrimaryKeyConstraint = "gigs_pkey"
)

// PgErrorTranslator converts pgx/Yugabyte errors into domain-aware repository
// errors.
type PgErrorTranslator struct{}

// NewPgErrorTranslator constructs the default Yugabyte error translator.
func NewPgErrorTranslator() DBErrorTranslator {
	return &PgErrorTranslator{}
}

func (t *PgErrorTranslator) TranslateCreateGigError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			if pgErr.ConstraintName == GigsPrimaryKeyConstraint {
				return WrapDomainError(domain.ErrGigAlreadyExists, err)
			}
		case pgerrcode.InvalidTextRepresentation:
			return WrapDomainError(domain.ErrInvalidGigID, err)
		}
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "SQLSTATE "+pgerrcode.UniqueViolation) {
		return WrapDomainError(domain.ErrGigAlreadyExists, err)
	}
	if strings.Contains(errMsg, "SQLSTATE "+pgerrcode.InvalidTextRepresentation) {
		return WrapDomainError(domain.ErrInvalidGigID, err)
	}

	return WrapCreateGigError(err)
}

func (t *PgErrorTranslator) TranslateFindGigError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return WrapDomainError(domain.ErrGigNotFound, err)
	}

	return WrapFindGigError(err)
}

func (t *PgErrorTranslator) TranslatePublishGigError(err error) error {
	return WrapPublishGigError(err)
}

// WrapDomainError preserves the domain error while attaching the original
// database cause for logging and debugging.
func WrapDomainError(domainErr, err error) error {
	return errors.Join(domainErr, err)
}
