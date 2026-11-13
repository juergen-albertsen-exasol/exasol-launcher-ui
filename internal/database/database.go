package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/exasol/exasol-driver-go"
	"github.com/exasol/exasol-driver-go/pkg/logger"
)

const (
	secretsFile = "secrets.json"
	pingTimeout = 5 * time.Second
	redacted    = "********"
	clientName  = "exasol-local-ui"
)

// Target identifies a database and the credentials to connect with.
type Target struct {
	Host                       string
	Port                       int
	Username                   string
	Password                   string
	InsecureSkipCertValidation bool
}

func (t Target) String() string {
	return t.Username + "@" + t.Host + ":" + strconv.Itoa(t.Port)
}

// ReadPassword returns the database password stored in the deployment
// directory's secrets.json.
func ReadPassword(deploymentDir string) (string, error) {
	path := filepath.Join(deploymentDir, secretsFile)

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read the database password: %w", err)
	}

	var secrets struct {
		DBPassword string `json:"dbPassword"`
	}

	// The decoding error is withheld because it may quote the file's content.
	if json.Unmarshal(content, &secrets) != nil {
		return "", fmt.Errorf("%s is not valid JSON", path)
	}

	if secrets.DBPassword == "" {
		return "", fmt.Errorf("%s contains no database password", path)
	}

	return secrets.DBPassword, nil
}

// Ping opens a fresh connection to target, runs SELECT 1, and returns the
// round-trip time of the query. Errors never contain the password.
func Ping(ctx context.Context, target Target) (time.Duration, error) {
	silenceDriverLogger()

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	roundTrip, err := querySelectOne(ctx, target)
	if err != nil {
		return 0, fmt.Errorf("cannot query the database as %s: %w",
			target, Redact(err, target.Password))
	}

	return roundTrip, nil
}

// Open returns a connection pool for target. Errors never contain the
// password.
func Open(target Target) (*sql.DB, error) {
	return openPool(target, true)
}

// OpenTransactional returns a connection pool for target whose connections
// commit only on COMMIT. Errors never contain the password.
func OpenTransactional(target Target) (*sql.DB, error) {
	return openPool(target, false)
}

func openPool(target Target, autocommit bool) (*sql.DB, error) {
	silenceDriverLogger()

	dsn := exasol.NewConfig(target.Username, target.Password).
		Host(target.Host).
		Port(target.Port).
		ValidateServerCertificate(!target.InsecureSkipCertValidation).
		ClientName(clientName).
		Autocommit(autocommit).
		String()

	connector, err := exasol.ExasolDriver{}.OpenConnector(dsn)
	if err != nil {
		return nil, Redact(err, target.Password)
	}

	return sql.OpenDB(connector), nil
}

func querySelectOne(ctx context.Context, target Target) (time.Duration, error) {
	connections, err := Open(target)
	if err != nil {
		return 0, err
	}
	defer connections.Close()

	conn, err := connections.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	var result int

	start := time.Now()
	if err := conn.QueryRowContext(ctx, "SELECT 1").Scan(&result); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

// Redact returns err with every occurrence of password masked.
func Redact(err error, password string) error {
	if err == nil || password == "" || !strings.Contains(err.Error(), password) {
		return err
	}

	return errors.New(strings.ReplaceAll(err.Error(), password, redacted))
}

type discardLogger struct{}

func (discardLogger) Print(...any)          {}
func (discardLogger) Printf(string, ...any) {}

// The driver logs connection errors itself, and these messages are returned
// to the caller anyway, where they pass through redaction.
var silenceDriverLogger = sync.OnceFunc(func() {
	_ = logger.SetLogger(discardLogger{})
})
