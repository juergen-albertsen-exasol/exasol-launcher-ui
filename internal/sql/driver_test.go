package sql_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/exasol/exasol-driver-go/pkg/types"
	"github.com/exasol/exasol-launcher-ui/internal/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseErrorsCarryCodeMessageAndPosition(t *testing.T) {
	t.Parallel()

	err := sql.StatementErrorOf(errors.New(
		"E-EGOD-11: execution failed with SQL error code '42000' and message "+
			"'object REGOIN_NAME not found [line 1, column 8] (Session: 1881751431478968320)'"),
		password)

	assert.Equal(t, &sql.StatementError{
		Code:    "42000",
		Message: "object REGOIN_NAME not found [line 1, column 8]",
		Line:    1,
		Column:  8,
	}, err)
}

func TestDatabaseErrorsWithoutPositionAndOtherErrors(t *testing.T) {
	t.Parallel()

	cancelled := sql.StatementErrorOf(errors.New(
		"E-EGOD-11: execution failed with SQL error code 'R0003' and message "+
			"'Client requested execution abort. (Session: 1)'"), password)
	other := sql.StatementErrorOf(errors.New("dial failed for "+password), password)

	assert.Equal(t, &sql.StatementError{
		Code: "R0003", Message: "Client requested execution abort.",
	}, cancelled)
	assert.EqualError(t, other, "dial failed for ********")
}

func TestValuesKeepTheDatabasesText(t *testing.T) {
	t.Parallel()

	for raw, expected := range map[string]string{
		`"12345678901234567890"`: "12345678901234567890",
		`123456789012345678`:     "123456789012345678",
		`1.5E10`:                 "1.5E10",
		`true`:                   "true",
		`"a \"quoted\" é"`:       `a "quoted" é`,
	} {
		value := sql.TextValue(json.RawMessage(raw))
		require.NotNil(t, value, raw)
		assert.Equal(t, expected, *value, raw)
	}

	assert.Nil(t, sql.TextValue(json.RawMessage(`null`)))
}

func TestColumnTypesAreWrittenAsExasolWritesThem(t *testing.T) {
	t.Parallel()

	number := func(value int64) *int64 { return &value }
	text := func(value string) *string { return &value }
	small := func(value int) *int { return &value }

	for expected, dataType := range map[string]types.SqlQueryColumnType{
		"DECIMAL(18,0)":     {Type: "DECIMAL", Precision: number(18), Scale: number(0)},
		"VARCHAR(100) UTF8": {Type: "VARCHAR", Size: number(100), CharacterSet: text("UTF8")},
		"CHAR(1) ASCII":     {Type: "CHAR", Size: number(1), CharacterSet: text("ASCII")},
		"HASHTYPE(16 BYTE)": {Type: "HASHTYPE", Size: number(32)},
		"GEOMETRY(4326)":    {Type: "GEOMETRY", Size: number(2_000_000), SRID: small(4326)},
		"INTERVAL DAY(4) TO SECOND(2)": {
			Type: "INTERVAL DAY TO SECOND", Precision: number(4), Fraction: small(2),
		},
		"INTERVAL YEAR(3) TO MONTH": {Type: "INTERVAL YEAR TO MONTH", Precision: number(3)},
		"TIMESTAMP":                 {Type: "TIMESTAMP", Size: number(16)},
		"TIMESTAMP WITH LOCAL TIME ZONE": {
			Type: "TIMESTAMP WITH LOCAL TIME ZONE", Size: number(16),
		},
		"DOUBLE":  {Type: "DOUBLE"},
		"BOOLEAN": {Type: "BOOLEAN"},
		"DATE":    {Type: "DATE", Size: number(4)},
	} {
		assert.Equal(t, expected, sql.FormatTypeOf(dataType))
	}
}
