# enum [![Build Status](https://github.com/go-pkgz/enum/workflows/build/badge.svg)](https://github.com/go-pkgz/enum/actions) [![Coverage Status](https://coveralls.io/repos/github/go-pkgz/enum/badge.svg?branch=master)](https://coveralls.io/github/go-pkgz/enum?branch=master) [![godoc](https://godoc.org/github.com/go-pkgz/enum?status.svg)](https://godoc.org/github.com/go-pkgz/enum)


`enum` is a Go package that provides a code generator for type-safe, json/bson/text-marshalable enumerations. It creates idiomatic Go code from simple type definitions, supporting both case-sensitive and case-insensitive string representations.

## Features

- Type-safe enum implementations
- JSON, BSON, SQL and text marshaling/unmarshaling support
- Case-sensitive or case-insensitive string representations
- Panic-free parsing with error handling
- Must-style parsing variants for convenience
- Easy value enumeration with Values() and Names() functions
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

### Generator Options

- `-type` (required): the name of the type to generate enum for (must be lowercase/private)
- `-path`: output directory path (default: same as source)
- `-lower`: use lowercase for string representations when marshaling/unmarshaling (affects only the output strings, not the naming pattern)
- `-getter`: enables the generation of an additional function, `Get{{Type}}ByID`, which attempts to find the corresponding enum element by its underlying integer ID. The `-getter` flag requires enum elements to have unique IDs to prevent undefined behavior.
- `-version`: print version information
- `-help`: show usage information

### Features of Generated Code

The generator creates a new type with the following features:

- String representation (implements `fmt.Stringer`)
- Text marshaling (implements `encoding.TextMarshaler` and `encoding.TextUnmarshaler`)
- Parse function with error handling (`ParseStatus`)
- Must-style parse function that panics on error (`MustStatus`)
- All possible values slice (`StatusValues`)
- All possible names slice (`StatusNames`)
- Go 1.23 iterator support (`StatusIter()`) for range-over-func syntax
- Public constants for each value (`StatusActive`, `StatusInactive`, etc.) - note that these are capitalized versions of your original constants

Additionally, if the `-getter` flag is set, a getter function (`GetStatusByID`) will be generated. This function allows retrieving an enum element using its raw integer ID.

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

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License