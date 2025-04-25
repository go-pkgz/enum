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

		gen, err = New("moreComplexType", "")
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

	t.Run("parse and generate with complex name", func(t *testing.T) {
		// create temp dir for output
		tmpDir := t.TempDir()

		gen, err := New("jobStatus", tmpDir)
		require.NoError(t, err)

		// parse testdata
		err = gen.Parse("testdata")
		require.NoError(t, err)

		// generate
		err = gen.Generate()
		require.NoError(t, err)

		// verify file was created
		content, err := os.ReadFile(filepath.Join(tmpDir, "job_status_enum.go"))
		require.NoError(t, err)

		// check content
		assert.Contains(t, string(content), "type JobStatus struct")
		assert.Contains(t, string(content), "JobStatusActive")
		assert.Contains(t, string(content), "JobStatusInactive")
		assert.Contains(t, string(content), "JobStatusBlocked")
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

	t.Run("explicit values", func(t *testing.T) {
		// create temp dir for output
		tmpDir := t.TempDir()

		gen, err := New("explicitValues", tmpDir)
		require.NoError(t, err)

		// parse testdata
		err = gen.Parse("testdata")
		require.NoError(t, err)

		// generate
		err = gen.Generate()
		require.NoError(t, err)

		// verify file was created
		content, err := os.ReadFile(filepath.Join(tmpDir, "explicit_values_enum.go"))
		require.NoError(t, err)

		// check content
		assert.Contains(t, string(content), "type ExplicitValues struct")
		assert.Contains(t, string(content), "value: 10") // should have actual value 10, not 0
		assert.Contains(t, string(content), "value: 20") // should have actual value 20, not 1
		assert.Contains(t, string(content), "value: 30") // should have actual value 30, not 2
	})

	t.Run("generate getter", func(t *testing.T) {
		// create temp dir for output
		tmpDir := t.TempDir()

		gen, err := New("jobStatus", tmpDir)
		require.NoError(t, err)
		gen.SetGenerateGetter(true)

		// parse testdata
		err = gen.Parse("testdata")
		require.NoError(t, err)

		// generate
		err = gen.Generate()
		require.NoError(t, err)

		// verify file was created
		content, err := os.ReadFile(filepath.Join(tmpDir, "job_status_enum.go"))
		require.NoError(t, err)

		// check content
		assert.Contains(t, string(content), "func GetJobStatusByID(v int) (JobStatus, error)")
		assert.Contains(t, string(content), "case 0:\n\t\treturn JobStatusUnknown, nil")
		assert.Contains(t, string(content), "case 1:\n\t\treturn JobStatusActive, nil")
		assert.Contains(t, string(content), "case 2:\n\t\treturn JobStatusInactive, nil")
		assert.Contains(t, string(content), "case 3:\n\t\treturn JobStatusBlocked, nil")
	})

	t.Run("generate getter explicit values", func(t *testing.T) {
		// create temp dir for output
		tmpDir := t.TempDir()

		gen, err := New("explicitValues", tmpDir)
		require.NoError(t, err)
		gen.SetGenerateGetter(true)

		// parse testdata
		err = gen.Parse("testdata")
		require.NoError(t, err)

		// generate
		err = gen.Generate()
		require.NoError(t, err)

		// verify file was created
		content, err := os.ReadFile(filepath.Join(tmpDir, "explicit_values_enum.go"))
		require.NoError(t, err)

		// check content
		assert.Contains(t, string(content), "func GetExplicitValuesByID(v int) (ExplicitValues, error)")
		assert.Contains(t, string(content), "case 10:\n\t\treturn ExplicitValuesFirst, nil")
		assert.Contains(t, string(content), "case 20:\n\t\treturn ExplicitValuesSecond, nil")
		assert.Contains(t, string(content), "case 30:\n\t\treturn ExplicitValuesThird, nil")
	})

	t.Run("generate getter repeated values", func(t *testing.T) {
		// create temp dir for output
		tmpDir := t.TempDir()

		gen, err := New("repeatValues", tmpDir)
		require.NoError(t, err)
		gen.SetGenerateGetter(true)

		// parse testdata
		err = gen.Parse("testdata")
		require.NoError(t, err)

		// generate
		err = gen.Generate()
		require.Error(t, err, "should fail with repeated values")
		assert.Contains(t, err.Error(), "multiple names for value 10: ")
		assert.Contains(t, err.Error(), "multiple names for value 20: ")
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

func TestRepeatValues(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := New("repeatValues", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	assert.Equal(t, 10, gen.values["repeatValuesFirst"], "First should be 10")
	assert.Equal(t, 10, gen.values["repeatValuesSecond"], "Second should repeat the value 10") // currently fails
	assert.Equal(t, 20, gen.values["repeatValuesThird"], "Third should be 20")
	assert.Equal(t, 20, gen.values["repeatValuesFourth"], "Fourth should repeat the value 20") // currently fails
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

func TestPermissions(t *testing.T) {
	t.Run("uses appropriate permissions", func(t *testing.T) {
		// create source directory with custom permissions
		tmpDir := t.TempDir()
		sourceDir := filepath.Join(tmpDir, "source")
		outputDir := filepath.Join(tmpDir, "output")

		// create source directory with 0755 permissions
		err := os.MkdirAll(sourceDir, 0o755)
		require.NoError(t, err)

		// create a sample status file
		sampleFile := `package source
const (
	statusUnknown = iota
	statusActive
	statusInactive
)
`
		err = os.WriteFile(filepath.Join(sourceDir, "status.go"), []byte(sampleFile), 0o644)
		require.NoError(t, err)

		// create generator and run it
		gen, err := New("status", outputDir)
		require.NoError(t, err)

		err = gen.Parse(sourceDir)
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		// check that output directory has same permissions as source directory
		outputInfo, err := os.Stat(outputDir)
		require.NoError(t, err)
		// on some OS TempDir may return different permissions, so we just check it's not 0700
		assert.NotEqual(t, os.FileMode(0o700), outputInfo.Mode().Perm(),
			"Output directory shouldn't have hardcoded 0o700 permissions")

		// check output file permissions
		outputFile := filepath.Join(outputDir, "status_enum.go")
		fileInfo, err := os.Stat(outputFile)
		require.NoError(t, err)
		// should be 0644 by default
		assert.Equal(t, os.FileMode(0o644), fileInfo.Mode().Perm(),
			"Output file should have 0o644 permissions")
	})
}

func TestNoLinterWarningsForUnusedConstants(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "linter_test.go"), []byte(`
package test
type linterTest uint8
const (
	linterTestUnknown linterTest = iota
	linterTestValue1
	linterTestValue2
)
`), 0o644)
	require.NoError(t, err)

	gen, err := New("linterTest", tmpDir)
	require.NoError(t, err)

	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)

	// read the generated file to check for the linter warning prevention code
	content, err := os.ReadFile(filepath.Join(tmpDir, "linter_test_enum.go"))
	require.NoError(t, err)

	// check that the unused constants prevention code exists
	assert.Contains(t, string(content), "// These variables are used to prevent the compiler from reporting unused errors")
	assert.Contains(t, string(content), "var _ = func() bool {")
	assert.Contains(t, string(content), "var _ linterTest = 0")
	assert.Contains(t, string(content), "var _ linterTest = linterTestUnknown")
	assert.Contains(t, string(content), "var _ linterTest = linterTestValue1")
	assert.Contains(t, string(content), "var _ linterTest = linterTestValue2")
	assert.Contains(t, string(content), "return true")
}

func TestGeneratorEdgeCases(t *testing.T) {
	t.Run("invalid template", func(t *testing.T) {
		// create a generator with a broken template that will fail to execute
		gen, err := New("status", "")
		require.NoError(t, err)

		// override template with invalid one
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

		// override template to generate invalid Go code
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

func TestSplitCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{""}},
		{"status", []string{"status"}},
		{"internalIPAddress", []string{"internal", "IP", "Address"}},
		{"internalIP", []string{"internal", "IP"}},
		{"HTTP", []string{"HTTP"}},
		{"HTTPResponseCode", []string{"HTTP", "Response", "Code"}},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := splitCamelCase(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestGetFileNameForType(t *testing.T) {
	tests := []struct {
		typeName string
		expected string
	}{
		{"status", "status_enum.go"},
		{"jobStatus", "job_status_enum.go"},
	}

	for _, test := range tests {
		t.Run(test.typeName, func(t *testing.T) {
			result := getFileNameForType(test.typeName)
			assert.Equal(t, test.expected, result)
		})
	}
}
