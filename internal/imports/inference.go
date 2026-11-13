// Package imports loads Parquet files into new tables. Its inference
// follows Fusion Cloud's port of Exasol's schema inference for Parquet, so
// both products suggest the same table for the same file.
package imports

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The Exasol types inference gives, and why it may give none.
const (
	Varchar = "VARCHAR(2000000) UTF8"
	Double  = "DOUBLE PRECISION"

	timestampType       = "TIMESTAMP(9)"
	maxDecimalPrecision = 36

	Nested          = "nested"
	UnsupportedType = "type"
)

// Names the Parquet specification gives types and annotations.
const (
	typeBoolean           = "BOOLEAN"
	typeInt32             = "INT32"
	typeInt64             = "INT64"
	typeInt96             = "INT96"
	typeFloat             = "FLOAT"
	typeDouble            = "DOUBLE"
	typeByteArray         = "BYTE_ARRAY"
	typeFixedLenByteArray = "FIXED_LEN_BYTE_ARRAY"

	annotationString          = "STRING"
	annotationUTF8            = "UTF8"
	annotationJSON            = "JSON"
	annotationBSON            = "BSON"
	annotationEnum            = "ENUM"
	annotationUUID            = "UUID"
	annotationFloat16         = "FLOAT16"
	annotationInterval        = "INTERVAL"
	annotationDecimal         = "DECIMAL"
	annotationDate            = "DATE"
	annotationTime            = "TIME"
	annotationTimestamp       = "TIMESTAMP"
	annotationInteger         = "INTEGER"
	annotationList            = "LIST"
	annotationMap             = "MAP"
	annotationMapKeyValue     = "MAP_KEY_VALUE"
	annotationTimeMillis      = "TIME_MILLIS"
	annotationTimeMicros      = "TIME_MICROS"
	annotationTimestampMillis = "TIMESTAMP_MILLIS"
	annotationTimestampMicros = "TIMESTAMP_MICROS"

	unitMillis = "MILLIS"
	unitMicros = "MICROS"
	unitNanos  = "NANOS"

	repeated = "REPEATED"
)

// Element is one element of a Parquet footer's flattened schema, with the
// names the Parquet specification gives its types.
type Element struct {
	Name           string
	Type           string
	RepetitionType string
	NumChildren    int
	ConvertedType  string
	Scale          int
	Precision      int
	LogicalType    *LogicalType
}

// LogicalType is an element's logical type annotation.
type LogicalType struct {
	Type            string
	Scale           int
	Precision       int
	Unit            string
	IsAdjustedToUTC bool
	BitWidth        int
	IsSigned        bool
}

// Column is a top-level Parquet column with the Exasol type that Exasol's
// inference gives it.
type Column struct {
	Name string
	// ParquetType is the type as people know it, such as INT64, STRING,
	// DECIMAL(9,3), or TIMESTAMP(us).
	ParquetType string
	// Type is the Exasol type, or empty when Exasol cannot import the column.
	Type string
	// Unsupported says why Exasol cannot import the column: Nested or
	// UnsupportedType.
	Unsupported string
}

// InferColumns returns the table's columns for a footer's flattened schema,
// whose first element is the root.
func InferColumns(schema []Element) []Column {
	if len(schema) == 0 {
		return nil
	}

	columns := make([]Column, 0, schema[0].NumChildren)
	index := 1

	for range schema[0].NumChildren {
		if index >= len(schema) {
			break
		}

		columns = append(columns, inferColumn(schema[index]))
		index = skip(schema, index)
	}

	return columns
}

// skip returns the index after the element at index and its descendants.
func skip(schema []Element, index int) int {
	next := index + 1
	for range schema[index].NumChildren {
		if next >= len(schema) {
			return next
		}

		next = skip(schema, next)
	}

	return next
}

func (e Element) logical() string {
	if e.LogicalType == nil {
		return ""
	}

	return e.LogicalType.Type
}

func inferColumn(element Element) Column {
	name := element.Name
	logical := element.logical()
	converted := element.ConvertedType

	if element.NumChildren > 0 {
		kind := "STRUCT"

		switch {
		case logical == annotationList || converted == annotationList:
			kind = annotationList
		case logical == annotationMap || converted == annotationMap ||
			converted == annotationMapKeyValue:
			kind = annotationMap
		default:
		}

		return Column{Name: name, ParquetType: kind, Unsupported: Nested}
	}

	// A repeated primitive is a list in the legacy encoding.
	if element.RepetitionType == repeated {
		return Column{Name: name, ParquetType: annotationList, Unsupported: Nested}
	}

	if logical == annotationDecimal || converted == annotationDecimal {
		precision, scale := element.Precision, element.Scale
		if logical == annotationDecimal {
			precision, scale = element.LogicalType.Precision, element.LogicalType.Scale
		}

		decimal := fmt.Sprintf("DECIMAL(%d,%d)", precision, scale)
		if precision > maxDecimalPrecision {
			return Column{Name: name, ParquetType: decimal, Unsupported: UnsupportedType}
		}

		return Column{Name: name, ParquetType: decimal, Type: decimal}
	}

	switch element.Type {
	case typeBoolean:
		return Column{Name: name, ParquetType: typeBoolean, Type: typeBoolean}
	case typeInt32, typeInt64:
		return integerOrTemporal(element)
	case typeInt96:
		return Column{Name: name, ParquetType: typeInt96, Type: timestampType}
	case typeFloat, typeDouble:
		return Column{Name: name, ParquetType: element.Type, Type: Double}
	case typeByteArray, typeFixedLenByteArray:
		return binary(element)
	default:
		parquetType := element.Type
		if parquetType == "" {
			parquetType = "UNKNOWN"
		}

		return Column{Name: name, ParquetType: parquetType, Unsupported: UnsupportedType}
	}
}

func binary(element Element) Column {
	name := element.Name
	logical := element.logical()
	converted := element.ConvertedType

	switch {
	case logical == annotationString || logical == annotationJSON || logical == annotationEnum ||
		converted == annotationUTF8 || converted == annotationJSON || converted == annotationEnum:
		return Column{Name: name, ParquetType: annotationString, Type: Varchar}
	case logical == annotationUUID:
		return Column{Name: name, ParquetType: annotationUUID, Unsupported: UnsupportedType}
	case logical == annotationFloat16:
		return Column{Name: name, ParquetType: annotationFloat16, Unsupported: UnsupportedType}
	case logical == annotationInterval || converted == annotationInterval:
		return Column{Name: name, ParquetType: annotationInterval, Unsupported: UnsupportedType}
	default:
		return Column{Name: name, ParquetType: "BINARY", Unsupported: UnsupportedType}
	}
}

var units = map[string]string{unitMillis: "ms", unitMicros: "us", unitNanos: "ns"}

var integerConversion = regexp.MustCompile(`^(U?)INT_(8|16|32|64)$`)

var integerDigits = map[int]int{8: 3, 16: 5, 32: 10, 64: 20} //nolint:mnd // Exasol's precisions

const (
	bits32 = 32
	bits64 = 64
)

func integerOrTemporal(element Element) Column {
	name := element.Name
	logical := element.logical()
	converted := element.ConvertedType

	switch {
	case logical == annotationDate || converted == annotationDate:
		return Column{Name: name, ParquetType: annotationDate, Type: annotationDate}
	case logical == annotationTimestamp:
		utc := ""
		if element.LogicalType.IsAdjustedToUTC {
			utc = ", UTC"
		}

		return Column{
			Name:        name,
			ParquetType: "TIMESTAMP(" + units[element.LogicalType.Unit] + utc + ")",
			Type:        timestampType,
		}
	case converted == annotationTimestampMillis:
		return Column{Name: name, ParquetType: "TIMESTAMP(ms, UTC)", Type: timestampType}
	case converted == annotationTimestampMicros:
		return Column{Name: name, ParquetType: "TIMESTAMP(us, UTC)", Type: timestampType}
	case logical == annotationTime:
		return Column{
			Name:        name,
			ParquetType: "TIME(" + units[element.LogicalType.Unit] + ")",
			Unsupported: UnsupportedType,
		}
	case converted == annotationTimeMillis:
		return Column{Name: name, ParquetType: "TIME(ms)", Unsupported: UnsupportedType}
	case converted == annotationTimeMicros:
		return Column{Name: name, ParquetType: "TIME(us)", Unsupported: UnsupportedType}
	default:
	}

	bits, signed := bits32, true
	if element.Type == typeInt64 {
		bits = bits64
	}

	if logical == annotationInteger {
		bits, signed = element.LogicalType.BitWidth, element.LogicalType.IsSigned
	} else if match := integerConversion.FindStringSubmatch(converted); match != nil {
		bits, _ = strconv.Atoi(match[2])
		signed = match[1] == ""
	}

	digits, known := integerDigits[bits]
	if !known {
		digits = integerDigits[bits64]
	}

	prefix := ""
	if !signed {
		prefix = "U"
	}

	return Column{
		Name:        name,
		ParquetType: prefix + "INT" + strconv.Itoa(bits),
		Type:        fmt.Sprintf("DECIMAL(%d,0)", digits),
	}
}

var (
	typeSpaces      = regexp.MustCompile(`\s+`)
	typePunctuation = regexp.MustCompile(`\s*([(),])\s*`)
)

// SameType reports whether two Exasol types name the same type, as the
// catalog and the suggestion write them.
func SameType(first, second string) bool {
	normal := func(dataType string) string {
		dataType = strings.ToUpper(dataType)
		dataType = typeSpaces.ReplaceAllString(dataType, " ")
		dataType = typePunctuation.ReplaceAllString(dataType, "$1")
		dataType = strings.TrimSpace(dataType)

		if dataType == "DOUBLE PRECISION" || dataType == typeFloat {
			return typeDouble
		}

		return dataType
	}

	return normal(first) == normal(second)
}

var (
	parquetExtension = regexp.MustCompile(`(?i)\.parquet$`)
	notIdentifier    = regexp.MustCompile(`[^A-Z0-9_]`)
	trailingOther    = regexp.MustCompile(`[^A-Za-z0-9]+$`)
)

// TableName returns the suggested table name for a file or folder name: its
// name without the .parquet extension, in uppercase, with every character
// other than letters, digits, and underscores replaced by an underscore.
func TableName(name string) string {
	name = strings.TrimRight(name, "/")
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		name = name[slash+1:]
	}

	stem := parquetExtension.ReplaceAllString(name, "")

	// Each character becomes one character, as in JavaScript, where a
	// replaced umlaut is one underscore.
	table := notIdentifier.ReplaceAllString(strings.ToUpper(stem), "_")
	if table == "" {
		return "IMPORT"
	}

	return table
}

// TableNameForFiles returns the table name for several files: their common
// beginning, such as orders-2025 for orders-2025-01.parquet to
// orders-2025-12.parquet, or the first file's name when they share none.
func TableNameForFiles(names []string) string {
	stems := make([]string, len(names))
	for index, name := range names {
		stems[index] = parquetExtension.ReplaceAllString(name, "")
	}

	if len(stems) <= 1 {
		if len(stems) == 0 {
			return TableName("")
		}

		return TableName(stems[0])
	}

	prefix := stems[0]
	for _, stem := range stems[1:] {
		for !strings.HasPrefix(stem, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}

	// Unlike Fusion Cloud, a prefix ending inside a word, such as 2024-0 for
	// 2024-01 and 2024-02, keeps only the whole words before it.
	if splitsWord(prefix, stems) {
		prefix = strings.TrimRightFunc(prefix, isAlphanumeric)
	}

	prefix = trailingOther.ReplaceAllString(prefix, "")
	if prefix == "" {
		return TableName(stems[0])
	}

	return TableName(prefix)
}

func isAlphanumeric(character rune) bool {
	return character < utf8.RuneSelf &&
		(unicode.IsLetter(character) || unicode.IsDigit(character))
}

// splitsWord reports whether prefix ends inside a word of one of the stems.
func splitsWord(prefix string, stems []string) bool {
	last, _ := utf8.DecodeLastRuneInString(prefix)
	if prefix == "" || !isAlphanumeric(last) {
		return false
	}

	for _, stem := range stems {
		next, _ := utf8.DecodeRuneInString(stem[len(prefix):])
		if len(stem) > len(prefix) && isAlphanumeric(next) {
			return true
		}
	}

	return false
}

// maxAlternative bounds the numbered alternatives to a taken table name.
const maxAlternative = 1000

// TableAlternatives returns the alternatives to a taken table name:
// <name>_2, <name>_3, and so on.
func TableAlternatives(name string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for number := 2; number < maxAlternative; number++ {
			if !yield(name + "_" + strconv.Itoa(number)) {
				return
			}
		}
	}
}

// SameColumns reports whether two files have the same columns: the same
// names and Parquet types, in the same order.
func SameColumns(first, second []Column) bool {
	if len(first) != len(second) {
		return false
	}

	for index := range first {
		if first[index].Name != second[index].Name ||
			first[index].ParquetType != second[index].ParquetType {
			return false
		}
	}

	return true
}

// UnsupportedMessage names the columns Exasol cannot import, and is empty
// when it can import them all.
func UnsupportedMessage(columns []Column) string {
	var nested, otherReason bool

	names := make([]string, 0, len(columns))

	for _, column := range columns {
		switch column.Unsupported {
		case "":
			continue
		case Nested:
			nested = true
		default:
			otherReason = true
		}

		names = append(names, column.Name+" ("+strings.ToLower(column.ParquetType)+")")
	}

	if len(names) == 0 {
		return ""
	}

	why := "binary, time, and UUID columns"

	switch {
	case nested && otherReason:
		why = "nested types, such as structs and lists, and binary, time, and UUID columns"
	case nested:
		why = "nested types, such as structs and lists"
	default:
	}

	subject, pronoun := "the columns", "them"
	if len(names) == 1 {
		subject, pronoun = "the column", "it"
	}

	return "Exasol cannot import " + subject + " " + strings.Join(names, ", ") +
		": it does not support " + why + ". Remove " + pronoun +
		" from the file, or import other files."
}
