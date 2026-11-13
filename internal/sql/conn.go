package sql

import (
	"context"
	"errors"
	"regexp"
	"strconv"

	"github.com/exasol/exasol-launcher-ui/internal/database"
)

// Column describes a column of a result set.
type Column struct {
	Name string
	Type string
}

// Value is a value in the database's text representation, or nil for NULL.
type Value = *string

// Rows is a statement's result set.
type Rows interface {
	Columns() []Column
	// Total is the number of rows the database announced.
	Total() int64
	// Next returns the next row, or io.EOF after the last one.
	Next(ctx context.Context) ([]Value, error)
}

// Conn is one connection to a database.
type Conn interface {
	// Execute runs statement. When it yields a result set, Execute passes it
	// to read and closes it afterwards; otherwise it returns the number of
	// affected rows. A failure the database reports is a *StatementError.
	Execute(ctx context.Context, statement string, read func(Rows) error) (int64, error)
	Close() error
}

// Opener connects to the databases of deployments.
type Opener interface {
	Open(ctx context.Context, deployment string, target database.Target) (Conn, error)
	Close() error
}

// StatementError is a failure the database reported for a statement. Code is
// empty when the statement failed outside the database, for example because
// the connection broke.
type StatementError struct {
	Code    string
	Message string
	// Line and Column locate the error within the statement when the
	// message names a position, and are zero otherwise.
	Line   int
	Column int
}

func (e *StatementError) Error() string {
	if e.Code == "" {
		return e.Message
	}

	return e.Code + ": " + e.Message
}

var (
	driverSQLError = regexp.MustCompile(`(?s)SQL error code '([^']*)' and message '(.*)'$`)
	sessionSuffix  = regexp.MustCompile(` \(Session: \d+\)$`)
	errorPosition  = regexp.MustCompile(`\[line (\d+), column (\d+)\]`)
)

// statementError turns an error the database reported through the driver
// into a *StatementError, and returns other errors unchanged.
func statementError(err error, password string) error {
	err = database.Redact(err, password)

	match := driverSQLError.FindStringSubmatch(err.Error())
	if match == nil {
		return err
	}

	statementErr := &StatementError{
		Code:    match[1],
		Message: sessionSuffix.ReplaceAllString(match[2], ""),
	}

	if position := errorPosition.FindStringSubmatch(statementErr.Message); position != nil {
		statementErr.Line, _ = strconv.Atoi(position[1])
		statementErr.Column, _ = strconv.Atoi(position[2])
	}

	return statementErr
}

// asStatementError returns err as a *StatementError, with a redacted message
// and no code when the database did not report it.
func asStatementError(err error, password string) *StatementError {
	if statementErr, ok := errors.AsType[*StatementError](err); ok {
		return statementErr
	}

	return &StatementError{Message: database.Redact(err, password).Error()}
}
