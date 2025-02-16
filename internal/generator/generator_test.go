package generator

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator(t *testing.T) {

	t.Run("validation", func(t *testing.T) {
		_, err := New("", "")
		require.Error(t, err, "empty type name should fail")

		_, err = New("Status", "")
		require.Error(t, err, "uppercase type name should fail")

		gen, err := New("status", "")
		require.NoError(t, err)
		assert.NotNil(t, gen)

		// check if generated code is valid Go code
		tmpDir := t.TempDir()
		gen, err = New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		// try to parse generated code
		fset := token.NewFileSet()
		genFile := filepath.Join(tmpDir, "status_enum.go")
		_, err = parser.ParseFile(fset, genFile, nil, parser.AllErrors)
		require.NoError(t, err, "generated code should be valid Go code")

		// validate default values correctness
		content, err := os.ReadFile(genFile)
		require.NoError(t, err)

		// check required imports
		assert.Contains(t, string(content), `"database/sql/driver"`)
		assert.Contains(t, string(content), `"fmt"`)

		// check required type definition
		assert.Contains(t, string(content), "type Status struct {")
		assert.Contains(t, string(content), "name  string")
		assert.Contains(t, string(content), "value int")

		// check all required methods are present
		methods := []string{
			"String() string",
			"MarshalText() ([]byte, error)",
			"UnmarshalText(text []byte) error",
			"Value() (driver.Value, error)",
			"Scan(value interface{}) error",
			"ParseStatus(v string) (Status, error)",
			"MustStatus(v string) Status",
			"StatusValues() []Status",
			"StatusNames() []string",
		}
		for _, method := range methods {
			assert.Contains(t, string(content), method, "method %s should be present", method)
		}
	})

	t.Run("parse and generate", func(t *testing.T) {
		// create temp dir for output
		tmpDir := t.TempDir()

		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		// parse testdata
		err = gen.Parse("testdata")
		require.NoError(t, err)

		// generate
		err = gen.Generate()
		require.NoError(t, err)

		// verify file was created
		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)

		// check content
		assert.Contains(t, string(content), "type Status struct")
		assert.Contains(t, string(content), "StatusActive")
		assert.Contains(t, string(content), "StatusInactive")
		assert.Contains(t, string(content), "StatusBlocked")
	})

	t.Run("sql support", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)

		// verify sql interface implementations are present
		assert.Contains(t, string(content), "func (e Status) Value() (driver.Value, error)")
		assert.Contains(t, string(content), "func (e *Status) Scan(value interface{}) error")

		// verify sql imports
		assert.Contains(t, string(content), `"database/sql/driver"`)

		// verify nil handling
		assert.Contains(t, string(content), "if value == nil {")
		assert.Contains(t, string(content), "StatusValues()[0]")

		// verify []byte support
		assert.Contains(t, string(content), "if b, ok := value.([]byte)")
	})

	t.Run("json support", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)

		// verify text marshaling interface implementations are present (used by json)
		assert.Contains(t, string(content), "func (e Status) MarshalText() ([]byte, error)")
		assert.Contains(t, string(content), "func (e *Status) UnmarshalText(text []byte) error")

		// verify proper error handling in unmarshal
		assert.Contains(t, string(content), "invalid status value: %v")
		assert.Contains(t, string(content), "ParseStatus(string(text))")

		// verify string conversion in marshal
		assert.Contains(t, string(content), "return []byte(e.name), nil")
	})

	t.Run("missing type", func(t *testing.T) {
		gen, err := New("nonexistent", "")
		require.NoError(t, err)

		err = gen.Parse("../testdata")
		assert.Error(t, err)
	})

	t.Run("invalid package", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "invalid.go"), []byte(`invalid go file`), 0o600)
		require.NoError(t, err)

		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse(tmpDir)
		assert.Error(t, err)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		gen, err := New("status", "")
		require.NoError(t, err)

		err = gen.Parse("non-existent-dir")
		assert.Error(t, err)
	})

	t.Run("invalid output directory", func(t *testing.T) {
		gen, err := New("status", "/non-existent-dir")
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		assert.Error(t, err)
	})
}

func TestGeneratorValues(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := New("status", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	assert.Equal(t, 0, gen.values["statusUnknown"], "unknown should be 0")
	assert.Equal(t, 1, gen.values["statusActive"], "active should be 1")
	assert.Equal(t, 2, gen.values["statusInactive"], "inactive should be 2")
	assert.Equal(t, 3, gen.values["statusBlocked"], "blocked should be 3")
}

func TestGeneratorSubdir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subpkg")
	require.NoError(t, os.MkdirAll(subDir, 0o700))

	gen, err := New("status", subDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)

	// verify file was created with correct package
	content, err := os.ReadFile(filepath.Join(subDir, "status_enum.go"))
	require.NoError(t, err)

	// should be package subpkg, not testdata
	assert.Contains(t, string(content), "package subpkg")
}

func TestGeneratorLowerCase(t *testing.T) {
	t.Run("lower case values", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "testenum")
		require.NoError(t, os.MkdirAll(subDir, 0o700))

		gen, err := New("status", subDir)
		require.NoError(t, err)
		gen.SetLowerCase(true)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(subDir, "status_enum.go"))
		require.NoError(t, err)

		// check string values are lowercase
		assert.Contains(t, string(content), `name: "active"`)
		assert.Contains(t, string(content), `name: "blocked"`)
		assert.Contains(t, string(content), `name: "inactive"`)
		assert.Contains(t, string(content), `name: "unknown"`)

		// check unmarshal code compares with lowercase
		assert.Contains(t, string(content), `case "active":`)
		assert.NotContains(t, string(content), "strings.ToLower")
	})

	t.Run("regular case values", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "testenum")
		require.NoError(t, os.MkdirAll(subDir, 0o700))

		gen, err := New("status", subDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(subDir, "status_enum.go"))
		require.NoError(t, err)

		// check string values are title case
		assert.Contains(t, string(content), `name: "Active"`)
		assert.Contains(t, string(content), `name: "Blocked"`)
		assert.Contains(t, string(content), "strings.ToLower")
	})
}

func TestGeneratorEdgeCases(t *testing.T) {
	t.Run("invalid template", func(t *testing.T) {
		// Create a generator with a broken template that will fail to execute
		gen, err := New("status", "")
		require.NoError(t, err)

		// Override template with invalid one
		origTmpl := enumTemplate
		defer func() { enumTemplate = origTmpl }()
		enumTemplate = template.Must(template.New("broken").Parse("{{.Unknown}}")) // will fail on execution

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute template")
	})

	t.Run("format error", func(t *testing.T) {
		gen, err := New("status", "")
		require.NoError(t, err)

		// Override template to generate invalid Go code
		origTmpl := enumTemplate
		defer func() { enumTemplate = origTmpl }()
		enumTemplate = template.Must(template.New("invalid").Parse("invalid go code"))

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to format source")
	})

	t.Run("invalid identifier", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected bool
		}{
			{"empty", "", false},
			{"starts with number", "123abc", false},
			{"valid", "abc123", true},
			{"valid with underscore", "abc_123", true},
			{"starts with underscore", "_abc123", true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.expected, isValidGoIdentifier(tc.input))
			})
		}
	})
}

func TestParseSpecialCases(t *testing.T) {
	t.Run("empty const block", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "empty.go"), []byte(`
package test
const (
)
`), 0o644)
		require.NoError(t, err)

		gen, err := New("status", "")
		require.NoError(t, err)

		err = gen.Parse(tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no const values found for type status")
	})

	t.Run("const without values", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := os.WriteFile(filepath.Join(tmpDir, "no_values.go"), []byte(`
package test
const name string
`), 0o644)
		require.NoError(t, err)

		gen, err := New("status", "")
		require.NoError(t, err)

		err = gen.Parse(tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no const values found for type status")
	})
}
