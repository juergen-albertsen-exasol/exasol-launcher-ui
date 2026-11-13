package imports

import (
	"context"
	"errors"
	"io"

	"github.com/exasol/exasol-local-ui/internal/sql"
)

func eachRow(ctx context.Context, rows sql.Rows, visit func([]sql.Value)) error {
	for {
		row, err := rows.Next(ctx)
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}

		visit(row)
	}
}
