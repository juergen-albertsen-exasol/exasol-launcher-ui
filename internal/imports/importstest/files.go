// Package importstest writes Parquet files for tests of imports.
package importstest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"
)

// Trip has columns of the Parquet types INT32, INT64, DOUBLE, and STRING.
type Trip struct {
	VendorID       int32   `parquet:"VendorID"`
	PassengerCount int64   `parquet:"passenger_count"`
	TripDistance   float64 `parquet:"trip_distance"`
	Flag           string  `parquet:"store_and_fwd_flag"`
}

// Event has a struct column payload, which Exasol cannot import.
type Event struct {
	ID      int64   `parquet:"id"`
	Payload Payload `parquet:"payload"`
}

type Payload struct {
	Kind string `parquet:"kind"`
}

// Trips returns count trips whose first one is {1, 2, 1.5, "N"}.
func Trips(count int) []Trip {
	const (
		vendors       = 2
		firstDistance = 1.5
	)

	trips := make([]Trip, count)
	for index := range trips {
		trips[index] = Trip{
			VendorID:       int32(index%vendors + 1), //nolint:gosec // small test values
			PassengerCount: int64(index%vendors + vendors),
			TripDistance:   firstDistance + float64(index),
			Flag:           "N",
		}
	}

	return trips
}

// Write writes rows as the Parquet file name in dir and returns its path.
func Write[T any](t *testing.T, dir, name string, rows []T) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := parquet.WriteFile(path, rows); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}

	return path
}

// Read returns the content of the file at path.
func Read(t *testing.T, path string) []byte {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}

	return content
}
