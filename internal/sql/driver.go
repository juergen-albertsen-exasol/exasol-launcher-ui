package sql

import (
	"context"
	dbsql "database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/exasol/exasol-driver-go/pkg/connection"
	"github.com/exasol/exasol-driver-go/pkg/types"
	"github.com/exasol/exasol-local-ui/internal/database"
)

const (
	resultSetType     = "resultSet"
	kibibyte          = 1024
	defaultFetchBytes = 2000 * kibibyte
	hexDigitsPerByte  = 2
	poolIdleTime      = 5 * time.Minute
)

// DriverOpener connects through exasol-driver-go, keeping one connection
// pool per deployment.
type DriverOpener struct {
	mutex sync.Mutex
	pools map[string]pool
}

type pool struct {
	target database.Target
	db     *dbsql.DB
}

func NewDriverOpener() *DriverOpener {
	return &DriverOpener{pools: map[string]pool{}}
}

func (o *DriverOpener) Open(
	ctx context.Context,
	deployment string,
	target database.Target,
) (Conn, error) {
	connections, err := o.pool(deployment, target)
	if err != nil {
		return nil, err
	}

	conn, err := connections.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to the database as %s: %w",
			target, database.Redact(err, target.Password))
	}

	return &driverConn{conn: conn, password: target.Password}, nil
}

func (o *DriverOpener) Close() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	errs := make([]error, 0, len(o.pools))
	for deployment, existing := range o.pools {
		errs = append(errs, existing.db.Close())
		delete(o.pools, deployment)
	}

	return errors.Join(errs...)
}

// OpenTransaction connects a connection of its own to target, with
// autocommit off, that runs IMPORT … FROM LOCAL statements by sending the
// files they name from this computer.
func (*DriverOpener) OpenTransaction(
	ctx context.Context,
	_ string,
	target database.Target,
) (Conn, error) {
	connections, err := database.OpenTransactional(target)
	if err != nil {
		return nil, err
	}

	connections.SetMaxOpenConns(1)

	conn, err := connections.Conn(ctx)
	if err != nil {
		_ = connections.Close()

		return nil, fmt.Errorf("cannot connect to the database as %s: %w",
			target, database.Redact(err, target.Password))
	}

	return &driverConn{conn: conn, password: target.Password, pool: connections}, nil
}

func (o *DriverOpener) pool(deployment string, target database.Target) (*dbsql.DB, error) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if existing, found := o.pools[deployment]; found {
		if existing.target == target {
			return existing.db, nil
		}

		_ = existing.db.Close()
		delete(o.pools, deployment)
	}

	connections, err := database.Open(target)
	if err != nil {
		return nil, err
	}

	connections.SetConnMaxIdleTime(poolIdleTime)
	o.pools[deployment] = pool{target: target, db: connections}

	return connections, nil
}

var localImport = regexp.MustCompile(`(?is)^\s*IMPORT\b.*\bFROM\s+LOCAL\b`)

// driverConn runs statements through the driver's exported connection rather
// than database/sql, which reports no row count for statements run as a
// query and no rows for statements run as an exec. It decodes values itself
// to keep the text the database sent, which the driver turns into float64.
type driverConn struct {
	conn     *dbsql.Conn
	password string
	// pool is set for a transaction's connection, which owns its pool and
	// runs local imports.
	pool *dbsql.DB
}

type executeResponse struct {
	ResultType string        `json:"resultType"`
	RowCount   int64         `json:"rowCount"`
	ResultSet  resultSetData `json:"resultSet"`
}

type resultSetData struct {
	ResultSetHandle int                    `json:"resultSetHandle"`
	NumRows         int64                  `json:"numRows"`
	Columns         []types.SqlQueryColumn `json:"columns"`
	// Data holds one slice of values per column.
	Data [][]json.RawMessage `json:"data"`
}

type fetchResponse struct {
	NumRows int                 `json:"numRows"`
	Data    [][]json.RawMessage `json:"data"`
}

func (c *driverConn) Execute(
	ctx context.Context,
	statement string,
	read func(Rows) error,
) (int64, error) {
	if c.pool != nil && localImport.MatchString(statement) {
		return c.importLocal(ctx, statement)
	}

	var rowCount int64

	err := c.conn.Raw(func(raw any) error {
		exa, ok := raw.(*connection.Connection)
		if !ok {
			return fmt.Errorf("unexpected driver connection %T", raw)
		}

		response, err := exa.SimpleExec(ctx, statement)
		if err != nil {
			return statementError(err, c.password)
		}

		var result executeResponse
		if err := json.Unmarshal(response.Results[0], &result); err != nil {
			return fmt.Errorf("cannot read the database's response: %w", err)
		}

		if result.ResultType != resultSetType {
			rowCount = result.RowCount

			return nil
		}

		rows := newDriverRows(exa, result.ResultSet, c.password)
		readErr := read(rows)

		return errors.Join(readErr, rows.close(ctx))
	})

	return rowCount, err
}

func (c *driverConn) Close() error {
	err := c.conn.Close()
	if c.pool != nil {
		err = errors.Join(err, c.pool.Close())
	}

	return err
}

// importLocal runs statement through the driver's own execution, which
// serves the local files it names.
func (c *driverConn) importLocal(ctx context.Context, statement string) (int64, error) {
	result, err := c.conn.ExecContext(ctx, statement)
	if err != nil {
		return 0, statementError(err, c.password)
	}

	return result.RowsAffected()
}

type driverRows struct {
	exa      *connection.Connection
	data     resultSetData
	columns  []Column
	password string
	// position counts the rows returned; offset indexes the current chunk.
	position int64
	offset   int
}

func newDriverRows(exa *connection.Connection, data resultSetData, password string) *driverRows {
	columns := make([]Column, len(data.Columns))
	for index, column := range data.Columns {
		columns[index] = Column{Name: column.Name, Type: formatType(column.DataType)}
	}

	return &driverRows{exa: exa, data: data, columns: columns, password: password}
}

func (r *driverRows) Columns() []Column {
	return r.columns
}

func (r *driverRows) Total() int64 {
	return r.data.NumRows
}

func (r *driverRows) Next(ctx context.Context) ([]Value, error) {
	if r.position >= r.data.NumRows || len(r.columns) == 0 {
		return nil, io.EOF
	}

	if len(r.data.Data) == 0 || r.offset >= len(r.data.Data[0]) {
		if err := r.fetch(ctx); err != nil {
			return nil, err
		}
	}

	row := make([]Value, len(r.data.Data))
	for column, values := range r.data.Data {
		row[column] = textValue(values[r.offset])
	}

	r.offset++
	r.position++

	return row, nil
}

func (r *driverRows) fetch(ctx context.Context) error {
	fetchBytes := defaultFetchBytes
	if r.exa.Config.FetchSize > 0 {
		fetchBytes = r.exa.Config.FetchSize * kibibyte
	}

	var chunk fetchResponse

	err := r.exa.Send(ctx, &types.FetchCommand{
		Command:         types.Command{Command: "fetch"},
		ResultSetHandle: r.data.ResultSetHandle,
		StartPosition:   int(r.position),
		NumBytes:        fetchBytes,
	}, &chunk)
	if err != nil {
		return statementError(err, r.password)
	}

	if chunk.NumRows == 0 || len(chunk.Data) != len(r.columns) {
		return errors.New("the database returned no further rows")
	}

	r.data.Data = chunk.Data
	r.offset = 0

	return nil
}

func (r *driverRows) close(ctx context.Context) error {
	if r.data.ResultSetHandle == 0 {
		return nil
	}

	return r.exa.Send(ctx, &types.CloseResultSetCommand{
		Command:          types.Command{Command: "closeResultSet"},
		ResultSetHandles: []int{r.data.ResultSetHandle},
	}, nil)
}

// textValue returns a value as the database wrote it: strings unquoted,
// numbers and booleans in their JSON text.
func textValue(raw json.RawMessage) Value {
	text := string(raw)
	if text == "null" {
		return nil
	}

	if unquoted, err := strconv.Unquote(text); err == nil {
		return &unquoted
	}

	var decoded string
	if json.Unmarshal(raw, &decoded) == nil {
		return &decoded
	}

	return &text
}

// formatType writes a column type as Exasol writes it in EXA_ALL_COLUMNS.
func formatType(dataType types.SqlQueryColumnType) string {
	name := dataType.Type

	switch name {
	case "DECIMAL":
		if dataType.Precision != nil && dataType.Scale != nil {
			return fmt.Sprintf("DECIMAL(%d,%d)", *dataType.Precision, *dataType.Scale)
		}
	case "VARCHAR", "CHAR":
		if dataType.Size != nil {
			formatted := fmt.Sprintf("%s(%d)", name, *dataType.Size)
			if dataType.CharacterSet != nil {
				formatted += " " + *dataType.CharacterSet
			}

			return formatted
		}
	case "HASHTYPE":
		if dataType.Size != nil {
			return fmt.Sprintf("HASHTYPE(%d BYTE)", *dataType.Size/hexDigitsPerByte)
		}
	case "GEOMETRY":
		if dataType.SRID != nil {
			return fmt.Sprintf("GEOMETRY(%d)", *dataType.SRID)
		}
	case "INTERVAL DAY TO SECOND":
		if dataType.Precision != nil && dataType.Fraction != nil {
			return fmt.Sprintf("INTERVAL DAY(%d) TO SECOND(%d)",
				*dataType.Precision, *dataType.Fraction)
		}
	case "INTERVAL YEAR TO MONTH":
		if dataType.Precision != nil {
			return fmt.Sprintf("INTERVAL YEAR(%d) TO MONTH", *dataType.Precision)
		}
	case "TIMESTAMP", "TIMESTAMP WITH LOCAL TIME ZONE":
		if dataType.Fraction != nil {
			return fmt.Sprintf("TIMESTAMP(%d)%s", *dataType.Fraction, name[len("TIMESTAMP"):])
		}
	default:
	}

	return name
}
