package imports

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/exasol/exasol-local-ui/internal/database"
	"github.com/exasol/exasol-local-ui/internal/sql"
)

// Schema is the schema imports suggest.
const Schema = "DATA"

// Codes of a source's errors and a suggestion's problems.
const (
	NotParquet         = "not_parquet"
	ColumnsDiffer      = "columns_differ"
	UnsupportedColumns = "unsupported_columns"
	TableTaken         = "table_taken"

	UploadSource = "upload"

	maxNamedFiles = 3
)

// ErrUnknownSource reports a source kind imports do not support.
var ErrUnknownSource = errors.New("imports support only the source kind upload")

// SourceError is why the files of a source cannot be imported together.
type SourceError struct {
	Code    string
	Message string
}

func (e *SourceError) Error() string {
	return e.Message
}

// Source names the files of an import: an upload's, for now.
type Source struct {
	Kind     string
	UploadID string
}

// SourceSummary describes a source's files.
type SourceSummary struct {
	Kind  string
	Label string
	Files int
	Bytes int64
}

// SuggestedColumn is a column of the suggested table.
type SuggestedColumn struct {
	Name        string
	Type        string
	ParquetType string
	Sample      *string
}

// Problem is a reason the suggestion cannot be imported as it is.
type Problem struct {
	Code        string
	Message     string
	Columns     []string
	Alternative string
}

// Suggestion is the table an import of a source creates.
type Suggestion struct {
	Source     SourceSummary
	Schema     string
	Table      string
	Rows       int64
	Columns    []SuggestedColumn
	Importable bool
	Problems   []Problem
}

// sourceFiles is a source's files, opened.
type sourceFiles struct {
	summary SourceSummary
	names   []string
	paths   []string
	files   []*File
	columns []Column
	rows    int64
}

func (s *sourceFiles) Close() {
	for _, file := range s.files {
		_ = file.Close()
	}
}

func label(names []string) string {
	if len(names) == 1 {
		return names[0]
	}

	return strconv.Itoa(len(names)) + " files"
}

func (s *sourceFiles) checkColumns() error {
	first := s.files[0].Columns

	var differing []string

	for index, file := range s.files {
		if !SameColumns(first, file.Columns) {
			differing = append(differing, s.names[index])
		}
	}

	if len(differing) == 0 {
		s.columns = first

		return nil
	}

	named := strings.Join(differing, ", ")
	if len(differing) > maxNamedFiles {
		named = strings.Join(differing[:maxNamedFiles], ", ") + ", and " +
			strconv.Itoa(len(differing)-maxNamedFiles) + " more"
	}

	verb := "have"
	if len(differing) == 1 {
		verb = "has"
	}

	return &SourceError{
		Code: ColumnsDiffer,
		Message: "The files of one import need the same columns, but " + named + " " + verb +
			" other columns than " + s.names[0] + ". Import them separately.",
	}
}

// Suggest returns the table an import of source into the database of
// deployment creates.
func (m *Manager) Suggest(
	ctx context.Context,
	deployment string,
	target database.Target,
	source Source,
) (Suggestion, error) {
	opened, err := m.open(deployment, source)
	if err != nil {
		return Suggestion{}, err
	}
	defer opened.Close()

	table := TableNameForFiles(opened.names)

	taken, err := m.takenNames(ctx, deployment, target, Schema)
	if err != nil {
		return Suggestion{}, err
	}

	samples := opened.files[0].Samples()
	columns := make([]SuggestedColumn, len(opened.columns))

	for index, column := range opened.columns {
		columns[index] = SuggestedColumn{
			Name: column.Name, Type: column.Type, ParquetType: column.ParquetType,
		}
		if samples != nil {
			columns[index].Sample = samples[index]
		}
	}

	problems := []Problem{}

	if message := UnsupportedMessage(opened.columns); message != "" {
		problem := Problem{Code: UnsupportedColumns, Message: message}

		for _, column := range opened.columns {
			if column.Unsupported != "" {
				problem.Columns = append(problem.Columns, column.Name)
			}
		}

		problems = append(problems, problem)
	}

	if taken[table] {
		alternative := availableTable(table, taken)
		problems = append(problems, Problem{
			Code: TableTaken,
			Message: Schema + "." + table + " exists. Choose another name, such as " +
				alternative + ".",
			Alternative: alternative,
		})
	}

	return Suggestion{
		Source:     opened.summary,
		Schema:     Schema,
		Table:      table,
		Rows:       opened.rows,
		Columns:    columns,
		Importable: len(problems) == 0,
		Problems:   problems,
	}, nil
}

// open opens the files of source, which must have the same columns.
func (m *Manager) open(deployment string, source Source) (*sourceFiles, error) {
	if source.Kind != UploadSource {
		return nil, fmt.Errorf("%w: %q", ErrUnknownSource, source.Kind)
	}

	upload, err := m.uploads.Get(deployment, source.UploadID)
	if err != nil {
		return nil, err
	}

	opened := &sourceFiles{
		summary: SourceSummary{Kind: UploadSource, Files: len(upload.Files)},
	}

	for index, spec := range upload.Files {
		path := m.uploads.Path(upload.ID, index)

		file, err := Open(path)
		if err != nil {
			opened.Close()

			if errors.Is(err, ErrNotParquet) {
				return nil, &SourceError{
					Code:    NotParquet,
					Message: spec.Name + " is not a Parquet file, or it is damaged.",
				}
			}

			return nil, err
		}

		opened.files = append(opened.files, file)
		opened.names = append(opened.names, spec.Name)
		opened.paths = append(opened.paths, path)
		opened.rows += file.Rows
		opened.summary.Bytes += spec.Size
	}

	opened.summary.Label = label(opened.names)

	if err := opened.checkColumns(); err != nil {
		opened.Close()

		return nil, err
	}

	return opened, nil
}

// availableTable returns the first of a taken table's alternatives that
// does not exist, such as TRIPS_2.
func availableTable(table string, taken map[string]bool) string {
	for alternative := range TableAlternatives(table) {
		if !taken[alternative] {
			return alternative
		}
	}

	return table
}

// TakenNamesQuery returns the query that lists the names of the tables and
// views of schema.
func TakenNamesQuery(schema string) string {
	literal := quoteLiteral(schema)

	return "SELECT TABLE_NAME FROM SYS.EXA_ALL_TABLES WHERE TABLE_SCHEMA = " + literal +
		" UNION ALL SELECT VIEW_NAME FROM SYS.EXA_ALL_VIEWS WHERE VIEW_SCHEMA = " + literal
}

func (m *Manager) takenNames(
	ctx context.Context,
	deployment string,
	target database.Target,
	schema string,
) (map[string]bool, error) {
	conn, err := m.opener.Open(ctx, deployment, target)
	if err != nil {
		return nil, database.Redact(err, target.Password)
	}
	defer conn.Close()

	taken := map[string]bool{}

	_, err = conn.Execute(ctx, TakenNamesQuery(schema), func(rows sql.Rows) error {
		return eachRow(ctx, rows, func(row []sql.Value) {
			if row[0] != nil {
				taken[*row[0]] = true
			}
		})
	})
	if err != nil {
		return nil, database.Redact(err, target.Password)
	}

	return taken, nil
}

// Quote returns identifier as a quoted SQL identifier.
func Quote(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func quoteLiteral(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}
