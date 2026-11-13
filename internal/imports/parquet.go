package imports

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/deprecated"
	"github.com/parquet-go/parquet-go/format"
)

const (
	// sampleTimeout bounds reading the first row; without it, the
	// suggestion has no samples.
	sampleTimeout   = 3 * time.Second
	maxSampleLength = 80
	secondsPerDay   = 24 * 60 * 60
	// unixJulianDay is the Julian day of 1970-01-01, which INT96 timestamps
	// count from.
	unixJulianDay = 2_440_588
	int96WordBits = 32
	bitsPerByte   = 8
)

// ErrNotParquet reports a file that is not a Parquet file or is damaged.
var ErrNotParquet = errors.New("not a Parquet file")

// Footer is what a Parquet file's footer says about it.
type Footer struct {
	Rows    int64
	Schema  []Element
	Columns []Column
}

// File is an opened Parquet file.
type File struct {
	Footer

	file   *parquet.File
	closer io.Closer
}

// Open reads the footer of the Parquet file at path. An error wraps
// ErrNotParquet when the file is not Parquet.
func Open(path string) (*File, error) {
	opened, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	info, err := opened.Stat()
	if err != nil {
		_ = opened.Close()

		return nil, err
	}

	file, err := parquet.OpenFile(opened, info.Size(),
		parquet.SkipPageIndex(true), parquet.SkipBloomFilters(true))
	if err != nil {
		_ = opened.Close()

		return nil, fmt.Errorf("%w: %w", ErrNotParquet, err)
	}

	schema := convertSchema(file.Metadata().Schema)

	return &File{
		Rows:    file.NumRows(),
		Schema:  schema,
		Columns: InferColumns(schema),
		file:    file,
		closer:  opened,
	}, nil
}

func (f *File) Close() error {
	return f.closer.Close()
}

// Samples returns the first row's value of each column as text, or nil
// when the file has no rows or reading the row takes too long. Columns
// Exasol cannot import, and NULL values, have no sample.
func (f *File) Samples() []*string {
	if f.Rows == 0 {
		return nil
	}

	result := make(chan []*string, 1)

	go func() {
		samples, err := f.readSamples()
		if err != nil {
			samples = nil
		}

		result <- samples
	}()

	timer := time.NewTimer(sampleTimeout)
	defer timer.Stop()

	select {
	case samples := <-result:
		return samples
	case <-timer.C:
		return nil
	}
}

func (f *File) readSamples() ([]*string, error) {
	row, err := f.firstRow()
	if err != nil {
		return nil, err
	}

	leaves := leafIndexes(f.Schema)
	elements := topLevelElements(f.Schema)
	samples := make([]*string, len(f.Columns))

	for index, column := range f.Columns {
		if column.Unsupported != "" || leaves[index] < 0 {
			continue
		}

		for _, value := range row {
			if value.Column() == leaves[index] {
				samples[index] = formatSample(value, elements[index], column)

				break
			}
		}
	}

	return samples, nil
}

func (f *File) firstRow() (row parquet.Row, err error) { //nolint:nonamedreturns // set on panic
	defer func() {
		// A damaged page can make the reader panic rather than fail.
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("cannot read the first row: %v", recovered)
		}
	}()

	groups := f.file.RowGroups()
	if len(groups) == 0 {
		return nil, io.EOF
	}

	rows := groups[0].Rows()
	defer rows.Close()

	buffer := make([]parquet.Row, 1)

	count, err := rows.ReadRows(buffer)
	if count == 0 {
		if err == nil {
			err = io.EOF
		}

		return nil, err
	}

	return buffer[0], nil
}

// leafIndexes returns the leaf column index of each top-level column, or -1
// for a nested one.
func leafIndexes(schema []Element) []int {
	if len(schema) == 0 {
		return nil
	}

	indexes := make([]int, 0, schema[0].NumChildren)
	leaf := 0
	index := 1

	for range schema[0].NumChildren {
		if index >= len(schema) {
			break
		}

		next := skip(schema, index)
		if schema[index].NumChildren > 0 {
			indexes = append(indexes, -1)

			for _, element := range schema[index+1 : next] {
				if element.NumChildren == 0 {
					leaf++
				}
			}
		} else {
			indexes = append(indexes, leaf)
			leaf++
		}

		index = next
	}

	return indexes
}

func topLevelElements(schema []Element) []Element {
	if len(schema) == 0 {
		return nil
	}

	elements := make([]Element, 0, schema[0].NumChildren)
	index := 1

	for range schema[0].NumChildren {
		if index >= len(schema) {
			break
		}

		elements = append(elements, schema[index])
		index = skip(schema, index)
	}

	return elements
}

func formatSample(value parquet.Value, element Element, column Column) *string {
	if value.IsNull() {
		return nil
	}

	text := sampleText(value, element, column)

	if utf8.RuneCountInString(text) > maxSampleLength {
		runes := []rune(text)
		text = string(runes[:maxSampleLength-1]) + "…"
	}

	return &text
}

func sampleText(value parquet.Value, element Element, column Column) string {
	switch {
	case strings.HasPrefix(column.Type, annotationDecimal) &&
		strings.HasPrefix(column.ParquetType, annotationDecimal):
		return decimalText(value, element)
	case column.Type == annotationDate:
		return time.Unix(int64(value.Int32())*secondsPerDay, 0).UTC().Format(time.DateOnly)
	case column.Type == timestampType:
		return timestampText(value, element)
	case column.Type == Varchar:
		return string(value.ByteArray())
	case column.Type == typeBoolean:
		return strconv.FormatBool(value.Boolean())
	case column.ParquetType == typeFloat:
		return strconv.FormatFloat(float64(value.Float()), 'f', -1, 32)
	case column.ParquetType == typeDouble:
		return strconv.FormatFloat(value.Double(), 'f', -1, 64)
	case strings.HasPrefix(column.ParquetType, "UINT"):
		if element.Type == typeInt64 {
			return strconv.FormatUint(uint64(value.Int64()), 10) //nolint:gosec // unsigned bits
		}

		return strconv.FormatUint(uint64(uint32(value.Int32())), 10) //nolint:gosec // unsigned bits
	case element.Type == typeInt64:
		return strconv.FormatInt(value.Int64(), 10)
	default:
		return strconv.FormatInt(int64(value.Int32()), 10)
	}
}

func decimalText(value parquet.Value, element Element) string {
	unscaled := new(big.Int)

	switch element.Type {
	case typeInt32:
		unscaled.SetInt64(int64(value.Int32()))
	case typeInt64:
		unscaled.SetInt64(value.Int64())
	default:
		bytes := value.ByteArray()
		unscaled.SetBytes(bytes)

		// The bytes are a big-endian two's complement number.
		if len(bytes) > 0 && bytes[0]&0x80 != 0 {
			unscaled.Sub(unscaled, new(big.Int).Lsh(big.NewInt(1), uint(len(bytes)*bitsPerByte)))
		}
	}

	scale := element.Scale
	if element.LogicalType != nil && element.LogicalType.Type == annotationDecimal {
		scale = element.LogicalType.Scale
	}

	if scale <= 0 {
		return unscaled.String()
	}

	digits := new(big.Int).Abs(unscaled).String()
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale-len(digits)+1) + digits
	}

	sign := ""
	if unscaled.Sign() < 0 {
		sign = "-"
	}

	return sign + digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
}

func timestampText(value parquet.Value, element Element) string {
	var instant time.Time

	unit := unitMillis
	if element.LogicalType != nil && element.LogicalType.Type == annotationTimestamp {
		unit = element.LogicalType.Unit
	} else if element.ConvertedType == annotationTimestampMicros {
		unit = unitMicros
	}

	switch {
	case element.Type == typeInt96:
		instant = int96Time(value.Int96())
	case unit == unitNanos:
		instant = time.Unix(0, value.Int64())
	case unit == unitMicros:
		instant = time.UnixMicro(value.Int64())
	default:
		instant = time.UnixMilli(value.Int64())
	}

	return instant.UTC().Format("2006-01-02 15:04:05.999999999")
}

func int96Time(value deprecated.Int96) time.Time {
	nanos := int64(value[1])<<int96WordBits | int64(value[0])
	days := int64(value[2]) - unixJulianDay

	return time.Unix(days*secondsPerDay, nanos)
}

var convertedTypes = map[deprecated.ConvertedType]string{
	deprecated.UTF8:            annotationUTF8,
	deprecated.Map:             annotationMap,
	deprecated.MapKeyValue:     annotationMapKeyValue,
	deprecated.List:            annotationList,
	deprecated.Enum:            annotationEnum,
	deprecated.Decimal:         annotationDecimal,
	deprecated.Date:            annotationDate,
	deprecated.TimeMillis:      annotationTimeMillis,
	deprecated.TimeMicros:      annotationTimeMicros,
	deprecated.TimestampMillis: annotationTimestampMillis,
	deprecated.TimestampMicros: annotationTimestampMicros,
	deprecated.Uint8:           "UINT_8",
	deprecated.Uint16:          "UINT_16",
	deprecated.Uint32:          "UINT_32",
	deprecated.Uint64:          "UINT_64",
	deprecated.Int8:            "INT_8",
	deprecated.Int16:           "INT_16",
	deprecated.Int32:           "INT_32",
	deprecated.Int64:           "INT_64",
	deprecated.Json:            annotationJSON,
	deprecated.Bson:            "BSON",
	deprecated.Interval:        annotationInterval,
}

var repetitionTypes = map[format.FieldRepetitionType]string{
	format.Required: "REQUIRED",
	format.Optional: "OPTIONAL",
	format.Repeated: repeated,
}

func convertSchema(schema []format.SchemaElement) []Element {
	elements := make([]Element, len(schema))

	for index, source := range schema {
		element := Element{Name: source.Name}

		if dataType, ok := source.Type.Get(); ok {
			element.Type = dataType.String()
		}

		if repetition, ok := source.RepetitionType.Get(); ok {
			element.RepetitionType = repetitionTypes[repetition]
		}

		if children, ok := source.NumChildren.Get(); ok {
			element.NumChildren = int(children)
		}

		if converted, ok := source.ConvertedType.Get(); ok {
			element.ConvertedType = convertedTypes[converted]
		}

		if scale, ok := source.Scale.Get(); ok {
			element.Scale = int(scale)
		}

		if precision, ok := source.Precision.Get(); ok {
			element.Precision = int(precision)
		}

		element.LogicalType = convertLogicalType(source.LogicalType)
		elements[index] = element
	}

	return elements
}

func convertLogicalType(logical format.LogicalType) *LogicalType {
	switch value := logical.Value.(type) {
	case *format.StringType:
		return &LogicalType{Type: annotationString}
	case *format.MapType:
		return &LogicalType{Type: annotationMap}
	case *format.ListType:
		return &LogicalType{Type: annotationList}
	case *format.EnumType:
		return &LogicalType{Type: annotationEnum}
	case *format.DecimalType:
		return &LogicalType{
			Type: annotationDecimal, Scale: int(value.Scale), Precision: int(value.Precision),
		}
	case *format.DateType:
		return &LogicalType{Type: annotationDate}
	case *format.TimeType:
		return &LogicalType{
			Type:            annotationTime,
			Unit:            timeUnit(value.Unit),
			IsAdjustedToUTC: value.IsAdjustedToUTC,
		}
	case *format.TimestampType:
		return &LogicalType{
			Type:            annotationTimestamp,
			Unit:            timeUnit(value.Unit),
			IsAdjustedToUTC: value.IsAdjustedToUTC,
		}
	case *format.IntType:
		return &LogicalType{
			Type: annotationInteger, BitWidth: int(value.BitWidth), IsSigned: value.IsSigned,
		}
	case *format.JsonType:
		return &LogicalType{Type: annotationJSON}
	case *format.BsonType:
		return &LogicalType{Type: annotationBSON}
	case *format.UUIDType:
		return &LogicalType{Type: annotationUUID}
	case *format.Float16Type:
		return &LogicalType{Type: annotationFloat16}
	case nil:
		return nil
	default:
		return &LogicalType{Type: strings.ToUpper(fmt.Sprintf("%T", value))}
	}
}

func timeUnit(unit format.TimeUnit) string {
	switch unit.Value.(type) {
	case *format.MilliSeconds:
		return unitMillis
	case *format.MicroSeconds:
		return unitMicros
	case *format.NanoSeconds:
		return unitNanos
	default:
		return ""
	}
}
