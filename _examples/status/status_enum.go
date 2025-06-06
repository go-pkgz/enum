// Code generated by enum generator; DO NOT EDIT.
package status

import (
	"fmt"

	"database/sql/driver"
)

// Status is the exported type for the enum
type Status struct {
	name  string
	value int
}

func (e Status) String() string { return e.name }

// MarshalText implements encoding.TextMarshaler
func (e Status) MarshalText() ([]byte, error) {
	return []byte(e.name), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (e *Status) UnmarshalText(text []byte) error {
	var err error
	*e, err = ParseStatus(string(text))
	return err
}

// Value implements the driver.Valuer interface
func (e Status) Value() (driver.Value, error) {
	return e.name, nil
}

// Scan implements the sql.Scanner interface
func (e *Status) Scan(value interface{}) error {
	if value == nil {
		*e = StatusValues()[0]
		return nil
	}

	str, ok := value.(string)
	if !ok {
		if b, ok := value.([]byte); ok {
			str = string(b)
		} else {
			return fmt.Errorf("invalid status value: %v", value)
		}
	}

	val, err := ParseStatus(str)
	if err != nil {
		return err
	}

	*e = val
	return nil
}

// ParseStatus converts string to status enum value
func ParseStatus(v string) (Status, error) {

	switch v {
	case "active":
		return StatusActive, nil
	case "blocked":
		return StatusBlocked, nil
	case "inactive":
		return StatusInactive, nil
	case "unknown":
		return StatusUnknown, nil

	}

	return Status{}, fmt.Errorf("invalid status: %s", v)
}

// MustStatus is like ParseStatus but panics if string is invalid
func MustStatus(v string) Status {
	r, err := ParseStatus(v)
	if err != nil {
		panic(err)
	}
	return r
}

// Public constants for status values
var (
	StatusActive   = Status{name: "active", value: 1}
	StatusBlocked  = Status{name: "blocked", value: 3}
	StatusInactive = Status{name: "inactive", value: 2}
	StatusUnknown  = Status{name: "unknown", value: 0}
)

// StatusValues returns all possible enum values
func StatusValues() []Status {
	return []Status{
		StatusActive,
		StatusBlocked,
		StatusInactive,
		StatusUnknown,
	}
}

// StatusNames returns all possible enum names
func StatusNames() []string {
	return []string{
		"active",
		"blocked",
		"inactive",
		"unknown",
	}
}

// StatusIter returns a function compatible with Go 1.23's range-over-func syntax.
// It yields all Status values in declaration order. Example:
//
//	for v := range StatusIter() {
//	    // use v
//	}
func StatusIter() func(yield func(Status) bool) {
	return func(yield func(Status) bool) {
		for _, v := range StatusValues() {
			if !yield(v) {
				break
			}
		}
	}
}

// These variables are used to prevent the compiler from reporting unused errors
// for the original enum constants. They are intentionally placed in a var block
// that is compiled away by the Go compiler.
var _ = func() bool {
	var _ status = 0
	// This avoids "defined but not used" linter error for statusActive
	var _ status = statusActive
	// This avoids "defined but not used" linter error for statusBlocked
	var _ status = statusBlocked
	// This avoids "defined but not used" linter error for statusInactive
	var _ status = statusInactive
	// This avoids "defined but not used" linter error for statusUnknown
	var _ status = statusUnknown
	return true
}()
