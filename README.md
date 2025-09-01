# enum [![Build Status](https://github.com/go-pkgz/enum/workflows/build/badge.svg)](https://github.com/go-pkgz/enum/actions) [![Coverage Status](https://coveralls.io/repos/github/go-pkgz/enum/badge.svg?branch=master)](https://coveralls.io/github/go-pkgz/enum?branch=master) [![godoc](https://godoc.org/github.com/go-pkgz/enum?status.svg)](https://godoc.org/github.com/go-pkgz/enum)


`enum` is a Go package that provides a code generator for type-safe, json/text-marshalable enumerations. Optional flags add SQL, BSON (MongoDB), and YAML support. It creates idiomatic Go code from simple type definitions, supporting both case-sensitive and case-insensitive string representations.

## Features

- Type-safe enum implementations
- Text marshaling/unmarshaling (JSON works via TextMarshaler)
- Optional SQL, BSON (MongoDB), and YAML support via flags
- Case-sensitive or case-insensitive string representations
- Panic-free parsing with error handling
- Must-style parsing variants for convenience
- Declaration order preservation (enums maintain source code order, not alphabetical)
- Type fidelity preservation (generated code uses the same underlying type as your enum)
- Optimized parsing with O(1) map-based lookups
- Smart SQL null handling (uses zero value when available, errors otherwise)
- Generated code is fully tested and documented
- No external runtime dependencies
- Supports Go 1.23's range-over-func iteration

## Quick Start

Here's a minimal example showing how to define and use an enum:

```go
//go:generate go run github.com/go-pkgz/enum@latest -type status -lower

// Type must be lowercase (private)
type status uint8

// Constants must be prefixed with the type name
const (
    statusUnknown status = iota
    statusActive
    statusInactive
    statusBlocked
)
```

Run `go generate` to create the enum implementation. The generated code provides:

```go
// use enum values
s := StatusActive
fmt.Println(s.String()) // prints: "active"

// parse from string
s, err := ParseStatus("active")
if err != nil {
    log.Fatal(err)
}

// use with JSON
type Data struct {
    Status Status `json:"status"`
}
d := Data{Status: StatusActive}
b, _ := json.Marshal(d)
fmt.Println(string(b)) // prints: {"status":"active"}

// iterate over all values using Go 1.23 range-over-func
for status := range StatusIter() {
    fmt.Println(status) // prints each status in turn
}
```

## Installation

```bash
go install github.com/go-pkgz/enum@latest
```

## Usage

1. Define your enum type and constants:

```go
// Type name must be lowercase (private)
type status uint8

// Constants must be prefixed with the type name
const (
    statusUnknown status = iota
    statusActive
    statusInactive
    statusBlocked
)
```

2. Add the generate directive:
```go
//go:generate go run github.com/go-pkgz/enum@latest -type status
```

3. Run the generator:
```bash
go generate ./...
```

By default the generated type supports `encoding.TextMarshaler`/`Unmarshaler` (used by `encoding/json`). To include other integrations, enable flags as needed (see below).

### Generator Options

- `-type` (required): the name of the type to generate enum for (must be lowercase/private)
- `-path`: output directory path (default: same as source)
- `-lower`: use lowercase for string representations when marshaling/unmarshaling
- `-getter`: enables the generation of an additional function, `Get{{Type}}ByID`, which attempts to find the corresponding enum element by its underlying integer ID. The `-getter` flag requires enum elements to have unique IDs to prevent undefined behavior.
- `-sql` (default: off): add SQL support via `database/sql/driver.Valuer` and `sql.Scanner`
- `-bson` (default: off): add MongoDB BSON support via `MarshalBSONValue`/`UnmarshalBSONValue`
- `-yaml` (default: off): add YAML support via `gopkg.in/yaml.v3` `Marshaler`/`Unmarshaler`
- `-version`: print version information
- `-help`: show usage information

### Features of Generated Code

The generator creates a new type with the following features:

- String representation (implements `fmt.Stringer`)
- Text marshaling (implements `encoding.TextMarshaler` and `encoding.TextUnmarshaler`)
- SQL support when `-sql` is set (implements `database/sql/driver.Valuer` and `sql.Scanner`)
- Parse function with error handling (`ParseStatus`) - uses efficient O(1) map lookup
- Must-style parse function that panics on error (`MustStatus`)
- All possible values as package variable (`StatusValues`) - preserves declaration order
- All possible names as package variable (`StatusNames`) - preserves declaration order
- Index method to get underlying integer value (`Status.Index()`)
- Go 1.23 iterator support (`StatusIter()`) for range-over-func syntax
- Public constants for each value (`StatusActive`, `StatusInactive`, etc.) - note that these are capitalized versions of your original constants

Additionally, if the `-getter` flag is set, a getter function (`GetStatusByID`) will be generated. This function allows retrieving an enum element using its raw integer ID.

### JSON, BSON, YAML

- JSON: works out of the box through `encoding.TextMarshaler`/`Unmarshaler`.
- BSON (MongoDB): enable `-bson` to generate `MarshalBSONValue`/`UnmarshalBSONValue`; values are stored as strings.
- YAML: enable `-yaml` to generate `MarshalYAML`/`UnmarshalYAML`; values are encoded as strings.

Example (MongoDB using `-bson`):

```go
//go:generate go run github.com/go-pkgz/enum@latest -type status -bson -lower

type status uint8
const (
    statusUnknown status = iota
    statusActive
    statusInactive
)

// Using mongo-go-driver
type User struct {
    ID     primitive.ObjectID `bson:"_id,omitempty"`
    Status Status             `bson:"status"`
}

// insert and read
u := User{Status: StatusActive}
_, _ = coll.InsertOne(ctx, u) // stores { status: "active" }

var out User
_ = coll.FindOne(ctx, bson.M{"status": "active"}).Decode(&out) // decodes via UnmarshalBSONValue
```

### Case Sensitivity

By default, the generator creates case-sensitive string representations. Use `-lower` flag for lowercase output:

```go
// default (case-sensitive)
StatusActive.String() // returns "Active"

// with -lower flag
StatusActive.String() // returns "active"
```

### Getter Generation

The `-getter` flag enables the generation of an additional function, `Get{{Type}}ByID`, which attempts to find the corresponding enum element by its underlying integer ID. If no matching element is found, an error is returned.

> **Note:**
> The `-getter` flag requires all IDs in the generated enum to be unique to prevent undefined behavior. If duplicate IDs are found, generation will fail with an error specifying which elements share the same ID.

### Error Handling

The generated Parse function includes proper error handling:

```go
status, err := ParseStatus("invalid")
if err != nil {
    // handle "invalid status: invalid" error
}

// or use Must variant if you're sure the value is valid
status := MustStatus("active") // panics if invalid
```

### SQL Database Support (with `-sql`)

The generated enums implement `database/sql/driver.Valuer` and `sql.Scanner` interfaces for seamless database integration:

```go
// Scanning from database
var s Status
err := db.QueryRow("SELECT status FROM users WHERE id = ?", userID).Scan(&s)

// Writing to database
_, err = db.Exec("UPDATE users SET status = ? WHERE id = ?", StatusActive, userID)

// Handling NULL values
// If the enum has a zero value (value = 0), NULL will scan to that value
// Otherwise, scanning NULL returns an error
```

### Performance Characteristics

- **Parsing**: O(1) constant time using map lookup (previously O(n) with switch statement)
- **Values/Names access**: Zero allocation - returns pre-computed package variables
- **Memory efficient**: Single shared instance for each enum value
- **Declaration order**: Preserved from source code, not alphabetically sorted

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License
