package sql

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/exasol/exasol-launcher-ui/internal/database"
)

const (
	SchemasQuery = "SELECT SCHEMA_NAME, SCHEMA_IS_VIRTUAL FROM SYS.EXA_ALL_SCHEMAS " +
		"ORDER BY SCHEMA_NAME"
	ObjectsQuery = "SELECT TABLE_SCHEMA, TABLE_NAME, 'TABLE' FROM SYS.EXA_ALL_TABLES " +
		"UNION ALL SELECT VIEW_SCHEMA, VIEW_NAME, 'VIEW' FROM SYS.EXA_ALL_VIEWS " +
		"ORDER BY 1, 2"

	tableKind = "table"
	viewKind  = "view"
)

var ErrNoSchema = errors.New("the database has no such schema")

// Schema is a schema of a database with its tables and views.
type Schema struct {
	Name    string
	Virtual bool
	Objects []Object
}

// Object is a table or a view; Columns is only filled when a schema's
// columns were read.
type Object struct {
	Name    string
	Kind    string
	Columns []Column
}

// SchemaQuery returns the query that finds the schema name.
func SchemaQuery(name string) string {
	return "SELECT SCHEMA_NAME, SCHEMA_IS_VIRTUAL FROM SYS.EXA_ALL_SCHEMAS " +
		"WHERE SCHEMA_NAME = " + quoteLiteral(name)
}

// ColumnsQuery returns the query that reads the columns of the schema name.
func ColumnsQuery(name string) string {
	return "SELECT COLUMN_TABLE, COLUMN_OBJECT_TYPE, COLUMN_NAME, COLUMN_TYPE " +
		"FROM SYS.EXA_ALL_COLUMNS WHERE COLUMN_SCHEMA = " + quoteLiteral(name) +
		" ORDER BY COLUMN_TABLE, COLUMN_ORDINAL_POSITION"
}

// Schemas lists the schemas of the database of deployment with their tables
// and views, on a short connection.
func (m *Manager) Schemas(
	ctx context.Context,
	deployment string,
	target database.Target,
) ([]Schema, error) {
	var schemas []Schema

	err := m.withConnection(ctx, deployment, target, func(conn Conn) error {
		schemaRows, err := queryAll(ctx, conn, SchemasQuery)
		if err != nil {
			return err
		}

		objectRows, err := queryAll(ctx, conn, ObjectsQuery)
		if err != nil {
			return err
		}

		schemas = make([]Schema, 0, len(schemaRows))
		indexes := make(map[string]int, len(schemaRows))

		for _, row := range schemaRows {
			indexes[text(row[0])] = len(schemas)
			schemas = append(schemas, Schema{
				Name:    text(row[0]),
				Virtual: text(row[1]) == "true",
				Objects: []Object{},
			})
		}

		for _, row := range objectRows {
			if index, found := indexes[text(row[0])]; found {
				schemas[index].Objects = append(schemas[index].Objects,
					Object{Name: text(row[1]), Kind: strings.ToLower(text(row[2]))})
			}
		}

		return nil
	})

	return schemas, err
}

// Schema reads the tables and views of the schema name with their columns,
// on a short connection.
func (m *Manager) Schema(
	ctx context.Context,
	deployment string,
	target database.Target,
	name string,
) (Schema, error) {
	var schema Schema

	err := m.withConnection(ctx, deployment, target, func(conn Conn) error {
		schemaRows, err := queryAll(ctx, conn, SchemaQuery(name))
		if err != nil {
			return err
		}

		if len(schemaRows) == 0 {
			return ErrNoSchema
		}

		columnRows, err := queryAll(ctx, conn, ColumnsQuery(name))
		if err != nil {
			return err
		}

		schema = Schema{Name: name, Virtual: text(schemaRows[0][1]) == "true", Objects: []Object{}}

		for _, row := range columnRows {
			last := len(schema.Objects) - 1
			if last < 0 || schema.Objects[last].Name != text(row[0]) {
				schema.Objects = append(schema.Objects, Object{
					Name: text(row[0]),
					Kind: objectKind(text(row[1])),
				})
				last++
			}

			schema.Objects[last].Columns = append(schema.Objects[last].Columns,
				Column{Name: text(row[2]), Type: text(row[3])})
		}

		return nil
	})

	return schema, err
}

func (m *Manager) withConnection(
	ctx context.Context,
	deployment string,
	target database.Target,
	use func(Conn) error,
) error {
	conn, err := m.opener.Open(ctx, deployment, target)
	if err != nil {
		return database.Redact(err, target.Password)
	}
	defer conn.Close()

	err = use(conn)
	if err == nil || errors.Is(err, ErrNoSchema) {
		return err
	}

	return asStatementError(err, target.Password)
}

func objectKind(objectType string) string {
	if objectType == "VIEW" {
		return viewKind
	}

	return tableKind
}

func queryAll(ctx context.Context, conn Conn, query string) ([][]Value, error) {
	var all [][]Value

	_, err := conn.Execute(ctx, query, func(rows Rows) error {
		for {
			row, err := rows.Next(ctx)
			if errors.Is(err, io.EOF) {
				return nil
			}

			if err != nil {
				return err
			}

			all = append(all, row)
		}
	})

	return all, err
}

func text(value Value) string {
	if value == nil {
		return ""
	}

	return *value
}

// quoteLiteral quotes value as a string literal.
func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
