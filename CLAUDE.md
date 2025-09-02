# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`enum` is a Go code generator that creates type-safe, marshalable enum implementations from simple type definitions. It generates idiomatic Go code with zero runtime dependencies and supports JSON, SQL, MongoDB BSON, and YAML marshaling through optional flags.

## Commands

### Build and Test
```bash
# Run all tests (excludes examples)
go test ./...

# Run tests with race detection and coverage
go test -race -cover ./...

# Run a specific test
go test -run TestRuntimeIntegration ./internal/generator

# Run integration tests (includes MongoDB container tests)
go test ./internal/generator -v -run TestRuntimeIntegration

# Build the enum generator
go build

# Install globally
go install github.com/go-pkgz/enum@latest
```

### Linting and Formatting
```bash
# Run linter (golangci-lint v2.0.2)
golangci-lint run

# Format code
gofmt -s -w .
goimports -w .
```

### Generate Enums
```bash
# Generate using go:generate directives
go generate ./...

# Direct invocation examples
go run github.com/go-pkgz/enum@latest -type status -lower
go run github.com/go-pkgz/enum@latest -type status -lower -sql -bson -yaml
```

## Architecture

### Core Components

1. **main.go** - CLI entry point that parses flags and invokes the generator
2. **internal/generator/generator.go** - Core generator logic:
   - Parses Go AST to find enum constants
   - Evaluates constant values including iota and binary expressions
   - Generates code from template with conditional blocks for features
3. **internal/generator/enum.go.tmpl** - Go template for generated code with conditional sections for SQL/BSON/YAML

### Key Design Decisions

1. **Type name must be lowercase (private)** - Enforced to prevent confusion with public types
2. **Constants must be prefixed with type name** - Ensures clear namespacing (e.g., `statusActive` for type `status`)
3. **Generated public types are capitalized** - Follows Go conventions (private `status` → public `Status`)
4. **Zero runtime dependencies** - Generated code uses only stdlib unless optional features are enabled
5. **Conditional feature generation** - SQL/BSON/YAML code only generated when flags are set to avoid forcing dependencies

### Integration Support

- **JSON**: Via `encoding.TextMarshaler`/`TextUnmarshaler` (always generated)
- **SQL** (`-sql` flag): Implements `database/sql/driver.Valuer` and `sql.Scanner`
  - Smart NULL handling: uses zero value if available, errors otherwise
- **BSON** (`-bson` flag): Implements `MarshalBSONValue`/`UnmarshalBSONValue` for MongoDB
  - Stores as string values, not documents
- **YAML** (`-yaml` flag): Implements `yaml.Marshaler`/`yaml.Unmarshaler` for gopkg.in/yaml.v3

### Testing Strategy

1. **Unit tests** (`generator_test.go`): Test parsing, generation, edge cases
2. **Integration tests** (`integration_test.go`):
   - `TestRuntimeIntegration`: Full pipeline test that builds binary, generates code, runs tests with real databases
   - Uses testcontainers for MongoDB integration testing
   - SQLite for SQL testing (in-memory)
3. **Test data** in `testdata/integration/`:
   - `enum_test.go`: Real database tests run by runtime integration
   - `status.go`, `priority.go`: Sample enums for testing

### Parsing and Generation Flow

1. Parse Go source files to find type definition
2. Extract constants prefixed with type name
3. Evaluate constant values:
   - Handle iota increments
   - Evaluate binary expressions (e.g., `iota + 1`, `1 << iota`)
   - Support explicit values
4. Generate code with:
   - String() method
   - Parse/Must functions
   - Marshal/Unmarshal methods based on flags
   - Iterator for Go 1.23+ range-over-func
   - Optional GetByID function (requires unique values)

## Important Constraints

1. **Enum type must be lowercase** - Generator validates and rejects uppercase type names
2. **Constants must start with type name** - e.g., for type `status`, constants must be `statusXxx`
3. **Unique IDs required for -getter flag** - Generator fails if duplicate values exist when getter is requested
4. **Declaration order preserved** - Enums maintain source order, not alphabetical
5. **Type fidelity** - Generated code preserves underlying type (uint8, int32, etc.)

## CI/CD

GitHub Actions workflow (`.github/workflows/ci.yml`):
- Runs on all pushes and PRs
- Tests with Go 1.24
- Runs tests with race detection and coverage
- Runs golangci-lint
- Submits coverage to Coveralls
- Tests examples separately

## Integration Testing Architecture

### Test Structure
The integration tests use a unique two-stage approach:

1. **`TestRuntimeIntegration`** in `integration_test.go`:
   - Builds the enum binary from source
   - Creates a temporary package with enum definitions
   - Runs the built binary to generate enum code
   - Creates a temporary `go.mod` with test dependencies
   - Copies test file from `testdata/integration/enum_test.go`
   - Runs the generated tests in isolation

2. **Actual database tests** in `testdata/integration/enum_test.go`:
   - `TestGeneratedEnumWithMongoDB`: Uses `github.com/go-pkgz/testutils` to spin up MongoDB 7 container
   - `TestGeneratedEnumWithSQL`: Uses in-memory SQLite for SQL testing
   - Tests real marshal/unmarshal operations, not just code generation

### Key Integration Test Patterns

1. **Test files in testdata are NOT compiled by Go** - They're copied and run in temp directory
2. **MongoDB container via testutils**:
   ```go
   mongoContainer := containers.NewMongoTestContainer(ctx, t, 7)
   defer mongoContainer.Close(ctx)
   coll := mongoContainer.Collection("test_db")
   ```
3. **Verifies storage format** - Confirms enums stored as strings in MongoDB, not empty documents
4. **Full round-trip testing** - Writes enum to database, reads back, verifies correct unmarshaling

### Running Integration Tests
```bash
# Run full integration test (builds binary, generates code, tests with real databases)
go test ./internal/generator -v -run TestRuntimeIntegration

# Skip integration tests in short mode
go test -short ./...

# Clean test cache before running to ensure fresh MongoDB container
go clean -testcache && go test ./internal/generator -v -run TestRuntimeIntegration
```

### Test Dependencies
Integration tests require:
- Docker for MongoDB containers (via testcontainers)
- Network access for go mod tidy in temp package
- Write access to temp directory for generated code

### Important Testing Details
- `TestMongoDBIntegration` in main `integration_test.go` only verifies code generation, not actual MongoDB
- Real MongoDB testing happens via `TestRuntimeIntegration` → `TestGeneratedEnumWithMongoDB`
- Tests verify both success and error paths (NULL handling, invalid values)
- Uses `require` for critical assertions, `assert` for non-critical ones