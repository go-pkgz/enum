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

## Quick Start

Here's a minimal example showing how to define and use an enum:

```go
//go:generate go run github.com/go-pkgz/enum -type status -lower

type status uint8

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
```

## Installation

```bash
go install github.com/go-pkgz/enum@latest
```

## Usage

1. Define your enum type and constants:

```go
type status uint8

const (
    statusUnknown status = iota
    statusActive
    statusInactive
    statusBlocked
)
```

2. Add the generate directive:
```go
//go:generate go run github.com/go-pkgz/enum -type status
```

3. Run the generator:
```bash
go generate ./...
```

### Generator Options

- `-type` (required): the name of the type to generate enum for (must be private)
- `-path`: output directory path (default: same as source)
- `-lower`: use lowercase for marshaled/unmarshaled values
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
- Public constants for each value (`StatusActive`, `StatusInactive`, etc.)

### Case Sensitivity

By default, the generator creates case-sensitive string representations. Use `-lower` flag for lowercase output:

```go
// default (case-sensitive)
StatusActive.String() // returns "Active"

// with -lower flag
StatusActive.String() // returns "active"
```

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