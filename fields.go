package ion

// FieldType roughly mirrors zapcore.FieldType
type FieldType uint8

const (
	UnknownType FieldType = iota
	StringType
	Int64Type
	Uint64Type
	Float64Type
	BoolType
	ErrorType
	AnyType
)

// Field represents a structured logging field (key-value pair).
// Field construction is zero-allocation for primitive types (String, Int, etc).
type Field struct {
	Key       string
	Type      FieldType
	Integer   int64
	StringVal string
	Float     float64
	Interface any
}

// F is a convenience constructor for Field.
// It detects the type and creates the appropriate Field.
func F(key string, value any) Field {
	switch v := value.(type) {
	case string:
		return String(key, v)
	case int:
		return Int(key, v)
	case int64:
		return Int64(key, v)
	case float64:
		return Float64(key, v)
	case bool:
		return Bool(key, v)
	case error:
		return Err(v)
	default:
		return Field{Key: key, Type: AnyType, Interface: value}
	}
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Type: StringType, StringVal: value}
}

// Int creates an integer field.
func Int(key string, value int) Field {
	return Field{Key: key, Type: Int64Type, Integer: int64(value)}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Type: Int64Type, Integer: value}
}

// Uint64 creates a uint64 field without truncation.
// Use this for large unsigned values (e.g., block heights, slots).
func Uint64(key string, value uint64) Field {
	return Field{Key: key, Type: Uint64Type, Interface: value}
}

// Float64 creates a float64 field.
func Float64(key string, value float64) Field {
	return Field{Key: key, Type: Float64Type, Float: value}
}

// Bool creates a boolean field.
func Bool(key string, value bool) Field {
	var i int64
	if value {
		i = 1
	}
	return Field{Key: key, Type: BoolType, Integer: i}
}

// Err creates an error field with the standard key "error".
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Type: AnyType, Interface: nil}
	}
	return Field{Key: "error", Type: ErrorType, Interface: err}
}
