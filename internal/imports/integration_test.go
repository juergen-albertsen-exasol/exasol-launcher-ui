//go:build integration

package imports_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/database"
	"github.com/exasol/exasol-launcher-ui/internal/imports"
	"github.com/exasol/exasol-launcher-ui/internal/imports/importstest"
	"github.com/exasol/exasol-launcher-ui/internal/launcher"
	"github.com/exasol/exasol-launcher-ui/internal/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	integrationSchema = "EXASOL_LAUNCHER_UI_IT_IMPORTS"
	sampleBucket      = "https://exasol-easy-data-access.s3.eu-central-1.amazonaws.com/sample-data/"
)

// localTarget returns the database of a local deployment whose database is
// ready, and skips the test when there is none.
func localTarget(t *testing.T) (string, database.Target) {
	t.Helper()

	if _, err := exec.LookPath(launcher.DefaultExecutable); err != nil {
		t.Skip("no launcher on PATH")
	}

	client := launcher.New(launcher.DefaultExecutable)

	deployments, err := client.Deployments(t.Context(), false)
	require.NoError(t, err)

	for _, deployment := range deployments {
		if deployment.Status != "database_ready" || deployment.Infrastructure != "local" {
			continue
		}

		info, err := client.Info(t.Context(), deployment.Path)
		require.NoError(t, err)
		require.NotNil(t, info.Connection)

		password, err := database.ReadPassword(deployment.Path)
		require.NoError(t, err)

		return deployment.Name, database.Target{
			Host:                       info.Connection.Host,
			Port:                       info.Connection.Port,
			Username:                   info.Connection.Username,
			Password:                   password,
			InsecureSkipCertValidation: info.Connection.InsecureSkipCertValidation,
		}
	}

	t.Skip("no local deployment with a ready database")

	return "", database.Target{}
}

func download(t *testing.T, name string) string {
	t.Helper()

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, sampleBucket+name, nil)
	require.NoError(t, err)

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)

	path := filepath.Join(t.TempDir(), name)
	output, err := os.Create(path)
	require.NoError(t, err)

	_, err = io.Copy(output, response.Body)
	require.NoError(t, err)
	require.NoError(t, output.Close())

	return path
}

func execute(t *testing.T, conn sql.Conn, statement string) {
	t.Helper()

	_, err := conn.Execute(t.Context(), statement, nil)
	require.NoError(t, err, statement)
}

func query(t *testing.T, conn sql.Conn, statement string) [][]string {
	t.Helper()

	var rows [][]string

	_, err := conn.Execute(t.Context(), statement, func(result sql.Rows) error {
		for {
			row, err := result.Next(t.Context())
			if errors.Is(err, io.EOF) {
				return nil
			}

			if err != nil {
				return err
			}

			texts := make([]string, len(row))
			for index, value := range row {
				if value != nil {
					texts[index] = *value
				}
			}

			rows = append(rows, texts)
		}
	})
	require.NoError(t, err, statement)

	return rows
}

func TestTheInferenceMatchesExasolsForTheSampleFiles(t *testing.T) {
	deployment, target := localTarget(t)
	opener := sql.NewDriverOpener()
	t.Cleanup(func() { _ = opener.Close() })

	for _, name := range []string{"online_products.parquet", "product_reviews.parquet"} {
		t.Run(name, func(t *testing.T) {
			file, err := imports.Open(download(t, name))
			require.NoError(t, err)
			t.Cleanup(func() { _ = file.Close() })

			conn, err := opener.OpenTransaction(t.Context(), deployment, target)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = conn.Execute(t.Context(), "ROLLBACK", nil)
				_ = conn.Close()
			})

			table := imports.Quote(integrationSchema) + "." + imports.Quote(imports.TableName(name))
			execute(t, conn, "CREATE SCHEMA IF NOT EXISTS "+imports.Quote(integrationSchema))
			execute(t, conn, "CREATE TABLE "+table+" AS (IMPORT FROM PARQUET AT '"+
				sampleBucket+"' FILE '"+name+"')")

			created := query(t, conn, "SELECT COLUMN_NAME, COLUMN_TYPE FROM SYS.EXA_ALL_COLUMNS "+
				"WHERE COLUMN_SCHEMA = '"+integrationSchema+"' AND COLUMN_TABLE = '"+
				imports.TableName(name)+"' ORDER BY COLUMN_ORDINAL_POSITION")

			require.Len(t, file.Columns, len(created))

			for index, column := range file.Columns {
				assert.Equal(t, created[index][0], column.Name)
				assert.True(t, imports.SameType(column.Type, created[index][1]),
					"%s: %s vs %s", column.Name, column.Type, created[index][1])
			}
		})
	}
}

type importFixture struct {
	t          *testing.T
	deployment string
	target     database.Target
	opener     *sql.DriverOpener
	uploads    *imports.Uploads
	manager    *imports.Manager
}

func newImportFixture(t *testing.T) *importFixture {
	t.Helper()

	deployment, target := localTarget(t)
	opener := sql.NewDriverOpener()
	uploads := imports.NewUploads(filepath.Join(t.TempDir(), "uploads"))
	manager := imports.NewManager(opener, uploads)

	fixture := &importFixture{
		t: t, deployment: deployment, target: target,
		opener: opener, uploads: uploads, manager: manager,
	}

	fixture.dropSchema(t.Context())
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		manager.Close(ctx)
		fixture.dropSchema(ctx)
		_ = uploads.Close()
		_ = opener.Close()
	})

	return fixture
}

func (f *importFixture) dropSchema(ctx context.Context) {
	f.t.Helper()

	conn, err := f.opener.Open(ctx, f.deployment, f.target)
	require.NoError(f.t, err)
	defer conn.Close()

	_, err = conn.Execute(ctx,
		"DROP SCHEMA IF EXISTS "+imports.Quote(integrationSchema)+" CASCADE", nil)
	require.NoError(f.t, err)
}

func (f *importFixture) run(statement string) [][]string {
	f.t.Helper()

	conn, err := f.opener.Open(f.t.Context(), f.deployment, f.target)
	require.NoError(f.t, err)
	defer conn.Close()

	return query(f.t, conn, statement)
}

func (f *importFixture) upload(path string) string {
	f.t.Helper()

	content := importstest.Read(f.t, path)
	created, err := f.uploads.Create(f.deployment, []imports.FileSpec{
		{Name: filepath.Base(path), Size: int64(len(content))},
	})
	require.NoError(f.t, err)
	require.NoError(f.t, f.uploads.Store(f.deployment, created.ID, 0,
		strings.NewReader(string(content))))

	return created.ID
}

func (f *importFixture) start(uploadID, table string, columns []imports.ColumnSpec) imports.Import {
	f.t.Helper()

	started, err := f.manager.Start(f.t.Context(), f.deployment, f.target, imports.Request{
		Source:  imports.Source{Kind: imports.UploadSource, UploadID: uploadID},
		Schema:  integrationSchema,
		Table:   table,
		Columns: columns,
	})
	require.NoError(f.t, err)

	return started
}

func (f *importFixture) finished(importID string) imports.Import {
	f.t.Helper()

	var found imports.Import

	require.Eventually(f.t, func() bool {
		var err error

		found, err = f.manager.Get(f.deployment, importID)
		require.NoError(f.t, err)

		return found.Status != imports.Running
	}, 2*time.Minute, 50*time.Millisecond)

	return found
}

func (f *importFixture) tableExists(table string) bool {
	f.t.Helper()

	return len(f.run("SELECT 1 FROM SYS.EXA_ALL_TABLES WHERE TABLE_SCHEMA = '"+
		integrationSchema+"' AND TABLE_NAME = '"+table+"'")) > 0
}

func tripColumns(passengerType string) []imports.ColumnSpec {
	return []imports.ColumnSpec{
		{Name: "VendorID", Type: "DECIMAL(10,0)"},
		{Name: "PASSENGERS", Type: passengerType},
		{Name: "trip_distance", Type: imports.Double},
		{Name: "store_and_fwd_flag", Type: imports.Varchar},
	}
}

func TestImportingWithChangedNamesAndTypesIntoTheDatabase(t *testing.T) {
	fixture := newImportFixture(t)
	path := importstest.Write(t, t.TempDir(), "trips.parquet", importstest.Trips(4))

	started := fixture.start(fixture.upload(path), "TRIPS", tripColumns("DECIMAL(3,0)"))
	done := fixture.finished(started.ID)

	require.Equal(t, imports.Succeeded, done.Status, done.Message)
	assert.Equal(t, int64(4), *done.Rows)
	assert.Equal(t, [][]string{{"PASSENGERS", "DECIMAL(3,0)"}}, fixture.run(
		"SELECT COLUMN_NAME, COLUMN_TYPE FROM SYS.EXA_ALL_COLUMNS WHERE COLUMN_SCHEMA = '"+
			integrationSchema+"' AND COLUMN_TABLE = 'TRIPS' AND COLUMN_ORDINAL_POSITION = 2"))
	assert.Equal(t, [][]string{{"10"}}, fixture.run(
		`SELECT SUM("PASSENGERS") FROM "`+integrationSchema+`"."TRIPS"`))
}

func TestAFailingImportLeavesNoTableInTheDatabase(t *testing.T) {
	fixture := newImportFixture(t)
	trips := importstest.Trips(2)
	trips[1].PassengerCount = 150
	path := importstest.Write(t, t.TempDir(), "trips.parquet", trips)

	started := fixture.start(fixture.upload(path), "TRIPS", tripColumns("DECIMAL(2,0)"))
	done := fixture.finished(started.ID)

	assert.Equal(t, imports.Failed, done.Status)
	assert.Contains(t, done.Message, "passenger_count")
	assert.False(t, fixture.tableExists("TRIPS"))
}

func TestCancellingAnImportLeavesNoTableInTheDatabase(t *testing.T) {
	fixture := newImportFixture(t)
	path := importstest.Write(t, t.TempDir(), "trips.parquet", importstest.Trips(3_000_000))

	started := fixture.start(fixture.upload(path), "TRIPS", nil)

	require.Eventually(t, func() bool {
		found, err := fixture.manager.Get(fixture.deployment, started.ID)
		require.NoError(t, err)

		return found.Files[0].Status == imports.Running
	}, time.Minute, 10*time.Millisecond)

	require.NoError(t, fixture.manager.Cancel(t.Context(), fixture.deployment, started.ID))

	done := fixture.finished(started.ID)

	assert.Equal(t, imports.Cancelled, done.Status)
	assert.False(t, fixture.tableExists("TRIPS"))
}
