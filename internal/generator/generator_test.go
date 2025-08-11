package generator

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
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
			"var StatusValues = []Status",
			"var StatusNames = []string",
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
		assert.Contains(t, string(content), "if v.Index() == 0")

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
		assert.Contains(t, string(content), "func GetJobStatusByID(v uint8) (JobStatus, error)")
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
		assert.Contains(t, string(content), "func GetExplicitValuesByID(v uint8) (ExplicitValues, error)")
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

	assert.Equal(t, 0, gen.values["statusUnknown"].value, "unknown should be 0")
	assert.Equal(t, 1, gen.values["statusActive"].value, "active should be 1")
	assert.Equal(t, 2, gen.values["statusInactive"].value, "inactive should be 2")
	assert.Equal(t, 3, gen.values["statusBlocked"].value, "blocked should be 3")
}

func TestRepeatValues(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := New("repeatValues", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	assert.Equal(t, 10, gen.values["repeatValuesFirst"].value, "First should be 10")
	assert.Equal(t, 10, gen.values["repeatValuesSecond"].value, "Second should repeat the value 10")
	assert.Equal(t, 20, gen.values["repeatValuesThird"].value, "Third should be 20")
	assert.Equal(t, 20, gen.values["repeatValuesFourth"].value, "Fourth should repeat the value 20")
}

func TestSQLNullHandling(t *testing.T) {
	t.Run("with zero value", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)

		// should scan nil to zero value when it exists
		assert.Contains(t, string(content), "if v.Index() == 0")
		assert.Contains(t, string(content), "*e = v")
		assert.Contains(t, string(content), "return nil")
	})

	t.Run("without zero value", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("noZero", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "no_zero_enum.go"))
		require.NoError(t, err)

		// should return error when no zero value exists
		assert.Contains(t, string(content), "cannot scan nil into NoZero: no zero value defined")
	})
}

func TestDeclarationOrderPreservation(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := New("orderTest", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	// generate the enum
	err = gen.Generate()
	require.NoError(t, err)

	// read the generated file
	content, err := os.ReadFile(filepath.Join(tmpDir, "order_test_enum.go"))
	require.NoError(t, err)

	// check that values appear in declaration order in Values() function
	// the order should be Zero, Alpha, Charlie, Bravo (not alphabetical)
	contentStr := string(content)

	// find the Values var and check order
	valuesIdx := strings.Index(contentStr, "var OrderTestValues = []OrderTest")
	require.GreaterOrEqual(t, valuesIdx, 0, "Should find OrderTestValues var")
	valuesSection := contentStr[valuesIdx : valuesIdx+300]

	// check order - Zero should come before Alpha, Alpha before Charlie, Charlie before Bravo
	zeroIdx := strings.Index(valuesSection, "OrderTestZero")
	alphaIdx := strings.Index(valuesSection, "OrderTestAlpha")
	charlieIdx := strings.Index(valuesSection, "OrderTestCharlie")
	bravoIdx := strings.Index(valuesSection, "OrderTestBravo")

	assert.Less(t, zeroIdx, alphaIdx, "Zero should come before Alpha")
	assert.Less(t, alphaIdx, charlieIdx, "Alpha should come before Charlie")
	assert.Less(t, charlieIdx, bravoIdx, "Charlie should come before Bravo (not alphabetical)")

	// find the Names var and check order
	namesIdx := strings.Index(contentStr, "var OrderTestNames = []string")
	require.GreaterOrEqual(t, namesIdx, 0, "Should find OrderTestNames var")
	namesSection := contentStr[namesIdx : namesIdx+200]

	// check order in names
	zeroNameIdx := strings.Index(namesSection, `"Zero"`)
	alphaNameIdx := strings.Index(namesSection, `"Alpha"`)
	charlieNameIdx := strings.Index(namesSection, `"Charlie"`)
	bravoNameIdx := strings.Index(namesSection, `"Bravo"`)

	assert.Less(t, zeroNameIdx, alphaNameIdx, "Zero name should come before Alpha")
	assert.Less(t, alphaNameIdx, charlieNameIdx, "Alpha name should come before Charlie")
	assert.Less(t, charlieNameIdx, bravoNameIdx, "Charlie name should come before Bravo (not alphabetical)")
}

func TestBinaryExprValues(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := New("binaryExpr", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	// check that all values are found
	assert.Contains(t, gen.values, "binaryExprFirst", "First value should be found")
	assert.Contains(t, gen.values, "binaryExprSecond", "Second value should be found")
	assert.Contains(t, gen.values, "binaryExprThird", "Third value should be found")

	// check that values are correct (iota + 1)
	assert.Equal(t, 1, gen.values["binaryExprFirst"].value, "First should be 1")
	assert.Equal(t, 2, gen.values["binaryExprSecond"].value, "Second should be 2")
	assert.Equal(t, 3, gen.values["binaryExprThird"].value, "Third should be 3")

	// generate the enum and verify it contains all constants
	err = gen.Generate()
	require.NoError(t, err)

	// verify file was created
	content, err := os.ReadFile(filepath.Join(tmpDir, "binary_expr_enum.go"))
	require.NoError(t, err)

	// check that all constants are present in the generated file
	assert.Contains(t, string(content), "BinaryExprFirst")
	assert.Contains(t, string(content), "BinaryExprSecond")
	assert.Contains(t, string(content), "BinaryExprThird")

	// check the values are correct
	assert.Contains(t, string(content), "value: 1")
	assert.Contains(t, string(content), "value: 2")
	assert.Contains(t, string(content), "value: 3")
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

		// check parse map has lowercase keys
		assert.Contains(t, string(content), `"active":   StatusActive`)
		// for lowercase mode, we don't use strings.ToLower in Parse function
		parseIdx := bytes.Index(content, []byte("func ParseStatus"))
		parseEnd := bytes.Index(content[parseIdx:], []byte("}"))
		parseFunc := string(content[parseIdx : parseIdx+parseEnd])
		assert.NotContains(t, parseFunc, "strings.ToLower")
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
	assert.Contains(t, string(content), "var _ linterTest = linterTest(0)")
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

func TestUnderlyingTypePreservation(t *testing.T) {
	t.Run("uint8 type", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// check that underlying type was captured
		assert.Equal(t, "uint8", gen.underlyingType)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)

		// verify that the generated code uses uint8
		assert.Contains(t, string(content), "value uint8")
		assert.Contains(t, string(content), "func (e Status) Index() uint8")
		assert.NotContains(t, string(content), "value int\n") // should not have plain int
	})

	t.Run("uint16 type", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("uint16Type", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		assert.Equal(t, "uint16", gen.underlyingType)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "uint16_type_enum.go"))
		require.NoError(t, err)

		assert.Contains(t, string(content), "value uint16")
		assert.Contains(t, string(content), "func (e Uint16Type) Index() uint16")
	})

	t.Run("int32 type", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("int32Type", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		assert.Equal(t, "int32", gen.underlyingType)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "int32_type_enum.go"))
		require.NoError(t, err)

		assert.Contains(t, string(content), "value int32")
		assert.Contains(t, string(content), "func (e Int32Type) Index() int32")
		// check that values are correct (100, 101)
		assert.Contains(t, string(content), "value: 100")
		assert.Contains(t, string(content), "value: 101")
	})

	t.Run("byte type alias", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("byteType", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// byte is an alias for uint8, but ast gives us "byte"
		assert.Equal(t, "byte", gen.underlyingType)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "byte_type_enum.go"))
		require.NoError(t, err)

		assert.Contains(t, string(content), "value byte")
		assert.Contains(t, string(content), "func (e ByteType) Index() byte")
	})

	t.Run("rune type alias", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("runeType", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// rune is an alias for int32, but ast gives us "rune"
		assert.Equal(t, "rune", gen.underlyingType)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "rune_type_enum.go"))
		require.NoError(t, err)

		assert.Contains(t, string(content), "value rune")
		assert.Contains(t, string(content), "func (e RuneType) Index() rune")
		// check that values are correct ('A' = 65, 'B' = 66)
		assert.Contains(t, string(content), "value: 65")
		assert.Contains(t, string(content), "value: 66")
	})

	t.Run("default int type", func(t *testing.T) {
		tmpDir := t.TempDir()

		// create a test file without explicit type
		testFile := `package test
const (
	someUnknown = iota
	someActive
)
`
		err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(testFile), 0o644)
		require.NoError(t, err)

		gen, err := New("some", tmpDir)
		require.NoError(t, err)

		err = gen.Parse(tmpDir)
		require.NoError(t, err)

		// check that underlying type is empty (will default to int)
		assert.Empty(t, gen.underlyingType)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "some_enum.go"))
		require.NoError(t, err)

		// verify that the generated code uses int as default
		assert.Contains(t, string(content), "value int")
		assert.Contains(t, string(content), "func (e Some) Index() int")
	})
}

func TestCaseInsensitiveParsing(t *testing.T) {
	tmpDir := t.TempDir()
	gen, err := New("status", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
	require.NoError(t, err)

	// verify that parsing uses strings.ToLower for case-insensitive matching
	assert.Contains(t, string(content), "strings.ToLower(v)")

	// verify the parse map has lowercase keys
	assert.Contains(t, string(content), `"unknown":  StatusUnknown`)
	assert.Contains(t, string(content), `"active":   StatusActive`)
	assert.Contains(t, string(content), `"inactive": StatusInactive`)
	assert.Contains(t, string(content), `"blocked":  StatusBlocked`)
}

func TestGeneratedCodeUsesVariables(t *testing.T) {
	tmpDir := t.TempDir()
	gen, err := New("status", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
	require.NoError(t, err)

	// verify that Values and Names are variables, not functions
	assert.Contains(t, string(content), "var StatusValues = []Status")
	assert.Contains(t, string(content), "var StatusNames = []string")

	// should NOT have function signatures
	assert.NotContains(t, string(content), "func StatusValues()")
	assert.NotContains(t, string(content), "func StatusNames()")

	// verify parse map is a variable
	assert.Contains(t, string(content), "var _statusParseMap = map[string]Status")
}

func TestGetterWithDifferentTypes(t *testing.T) {
	t.Run("getter with uint16", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("uint16Type", tmpDir)
		require.NoError(t, err)
		gen.SetGenerateGetter(true)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "uint16_type_enum.go"))
		require.NoError(t, err)

		// verify getter uses uint16
		assert.Contains(t, string(content), "func GetUint16TypeByID(v uint16) (Uint16Type, error)")
	})

	t.Run("getter with int32", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("int32Type", tmpDir)
		require.NoError(t, err)
		gen.SetGenerateGetter(true)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "int32_type_enum.go"))
		require.NoError(t, err)

		// verify getter uses int32
		assert.Contains(t, string(content), "func GetInt32TypeByID(v int32) (Int32Type, error)")
		// verify it has correct values
		assert.Contains(t, string(content), "case 100:")
		assert.Contains(t, string(content), "case 101:")
	})
}

func TestBinaryExpressionEdgeCases(t *testing.T) {
	t.Run("multiplication with iota", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("mulDivType", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// check values
		assert.Equal(t, 0, gen.values["mulDivTypeA"].value)
		assert.Equal(t, 2, gen.values["mulDivTypeB"].value)
		assert.Equal(t, 4, gen.values["mulDivTypeC"].value)
	})

	t.Run("right-side iota addition", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("rightIotaType", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// check values
		assert.Equal(t, 10, gen.values["rightIotaTypeX"].value)
		assert.Equal(t, 11, gen.values["rightIotaTypeY"].value)
	})

	t.Run("subtraction with iota", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("subType", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// check values
		assert.Equal(t, 100, gen.values["subTypeA"].value)
		assert.Equal(t, 99, gen.values["subTypeB"].value)
		assert.Equal(t, 98, gen.values["subTypeC"].value)
	})
}

func TestConvertLiteralToInt(t *testing.T) {
	tests := []struct {
		name      string
		literal   *ast.BasicLit
		expected  int
		expectErr bool
	}{
		{
			name:     "integer literal",
			literal:  &ast.BasicLit{Kind: token.INT, Value: "42"},
			expected: 42,
		},
		{
			name:     "character literal single quote",
			literal:  &ast.BasicLit{Kind: token.CHAR, Value: "'A'"},
			expected: 65,
		},
		{
			name:     "character literal escape",
			literal:  &ast.BasicLit{Kind: token.CHAR, Value: "'\\n'"},
			expected: 10,
		},
		{
			name:      "invalid integer format",
			literal:   &ast.BasicLit{Kind: token.INT, Value: "not_a_number"},
			expectErr: true,
		},
		{
			name:      "multi-character literal",
			literal:   &ast.BasicLit{Kind: token.CHAR, Value: "'AB'"},
			expectErr: true,
		},
		{
			name:      "invalid character literal",
			literal:   &ast.BasicLit{Kind: token.CHAR, Value: "invalid"},
			expectErr: true,
		},
		{
			name:      "unsupported literal kind",
			literal:   &ast.BasicLit{Kind: token.FLOAT, Value: "3.14"},
			expectErr: true,
		},
		{
			name:     "character literal tab",
			literal:  &ast.BasicLit{Kind: token.CHAR, Value: "'\\t'"},
			expected: 9,
		},
		{
			name:     "character literal null",
			literal:  &ast.BasicLit{Kind: token.CHAR, Value: "'\\x00'"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertLiteralToInt(tt.literal)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestUnderscorePlaceholderConstants(t *testing.T) {
	// test that underscore placeholders are skipped
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
	type status int
	const (
		statusFirst = iota
		_  // skip this value
		statusSecond
		_  // skip this too
		statusThird
	)`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("status", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	// check that underscore placeholders were skipped but iota still incremented
	assert.Equal(t, 0, gen.values["statusFirst"].value)
	assert.Equal(t, 2, gen.values["statusSecond"].value) // iota=2 (after _ at iota=1)
	assert.Equal(t, 4, gen.values["statusThird"].value)  // iota=4 (after _ at iota=3)
	_, exists := gen.values["_"]
	assert.False(t, exists, "underscore should not be in values")
}

func TestDivisionOperationsWithIota(t *testing.T) {
	// test division operations in applyIotaOperation
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
	type divType int
	const (
		divTypeA = iota / 2
		divTypeB
		divTypeC
		divTypeD
	)`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("divType", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	// iota/2: 0/2=0, 1/2=0, 2/2=1, 3/2=1
	assert.Equal(t, 0, gen.values["divTypeA"].value)
	assert.Equal(t, 0, gen.values["divTypeB"].value)
	assert.Equal(t, 1, gen.values["divTypeC"].value)
	assert.Equal(t, 1, gen.values["divTypeD"].value)
}

func TestSubtractionWithIota(t *testing.T) {
	// test subtraction operations - both iota - N and N - iota
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
	type subType int
	const (
		subTypeA = 10 - iota  // 10 - 0 = 10
		subTypeB              // 10 - 1 = 9
		subTypeC              // 10 - 2 = 8
		subTypeD = iota - 1   // 3 - 1 = 2
		subTypeE              // 4 - 1 = 3
	)`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("subType", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 10, gen.values["subTypeA"].value)
	assert.Equal(t, 9, gen.values["subTypeB"].value)
	assert.Equal(t, 8, gen.values["subTypeC"].value)
	assert.Equal(t, 2, gen.values["subTypeD"].value)
	assert.Equal(t, 3, gen.values["subTypeE"].value)
}

func TestEmptyConstBlock(t *testing.T) {
	// test handling of empty const blocks
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
	type emptyType int
	const (
		// this const block has no values
	)
	const (
		emptyTypeFirst = iota
	)`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("emptyType", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 0, gen.values["emptyTypeFirst"].value)
}

func TestZeroBinaryExpression(t *testing.T) {
	// test a binary expression that evaluates to 0 without iota
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
	type zeroType int
	const (
		zeroTypeA = 5 - 5  // plain binary expr that equals 0
		zeroTypeB = iota   // should be 1
	)`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("zeroType", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 0, gen.values["zeroTypeA"].value)
	assert.Equal(t, 1, gen.values["zeroTypeB"].value)
}

func TestEvaluateBinaryExpr(t *testing.T) {
	tests := []struct {
		name         string
		expr         *ast.BinaryExpr
		iotaVal      int
		expectedVal  int
		expectedIota bool
		expectErr    bool
	}{
		{
			name: "iota + 1",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "iota"},
				Op: token.ADD,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "1"},
			},
			iotaVal:      0,
			expectedVal:  1,
			expectedIota: true,
		},
		{
			name: "iota * 2",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "iota"},
				Op: token.MUL,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "2"},
			},
			iotaVal:      3,
			expectedVal:  6,
			expectedIota: true,
		},
		{
			name: "100 - iota",
			expr: &ast.BinaryExpr{
				X:  &ast.BasicLit{Kind: token.INT, Value: "100"},
				Op: token.SUB,
				Y:  &ast.Ident{Name: "iota"},
			},
			iotaVal:      2,
			expectedVal:  98,
			expectedIota: true,
		},
		{
			name: "iota - 5",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "iota"},
				Op: token.SUB,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "5"},
			},
			iotaVal:      10,
			expectedVal:  5,
			expectedIota: true,
		},
		{
			name: "10 + iota",
			expr: &ast.BinaryExpr{
				X:  &ast.BasicLit{Kind: token.INT, Value: "10"},
				Op: token.ADD,
				Y:  &ast.Ident{Name: "iota"},
			},
			iotaVal:      2,
			expectedVal:  12,
			expectedIota: true,
		},
		{
			name: "iota / 2",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "iota"},
				Op: token.QUO,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "2"},
			},
			iotaVal:      4,
			expectedVal:  2,
			expectedIota: true,
		},
		{
			name: "division by zero",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "iota"},
				Op: token.QUO,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "0"},
			},
			iotaVal:   1,
			expectErr: true,
		},
		{
			name: "unsupported operator",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "iota"},
				Op: token.REM,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "2"},
			},
			iotaVal:   1,
			expectErr: true,
		},
		{
			name: "unsupported left identifier",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "unknown"},
				Op: token.ADD,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "1"},
			},
			iotaVal:   0,
			expectErr: true,
		},
		{
			name: "unsupported right identifier",
			expr: &ast.BinaryExpr{
				X:  &ast.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.ADD,
				Y:  &ast.Ident{Name: "unknown"},
			},
			iotaVal:   0,
			expectErr: true,
		},
		{
			name: "invalid left literal",
			expr: &ast.BinaryExpr{
				X:  &ast.BasicLit{Kind: token.INT, Value: "invalid"},
				Op: token.ADD,
				Y:  &ast.Ident{Name: "iota"},
			},
			iotaVal:   0,
			expectErr: true,
		},
		{
			name: "invalid right literal",
			expr: &ast.BinaryExpr{
				X:  &ast.Ident{Name: "iota"},
				Op: token.ADD,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "invalid"},
			},
			iotaVal:   0,
			expectErr: true,
		},
		{
			name: "unsupported left type",
			expr: &ast.BinaryExpr{
				X:  &ast.CallExpr{},
				Op: token.ADD,
				Y:  &ast.BasicLit{Kind: token.INT, Value: "1"},
			},
			iotaVal:   0,
			expectErr: true,
		},
		{
			name: "unsupported right type",
			expr: &ast.BinaryExpr{
				X:  &ast.BasicLit{Kind: token.INT, Value: "1"},
				Op: token.ADD,
				Y:  &ast.CallExpr{},
			},
			iotaVal:   0,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, usesIota, err := EvaluateBinaryExpr(tt.expr, tt.iotaVal)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedVal, val)
				assert.Equal(t, tt.expectedIota, usesIota)
			}
		})
	}
}

func TestApplyIotaOperationNil(t *testing.T) {
	gen, err := New("test", "")
	require.NoError(t, err)

	// test nil operation returns iotaVal unchanged
	result := gen.applyIotaOperation(nil, 42)
	assert.Equal(t, 42, result)
}

func TestDivisionByZeroInQUO(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type divZero int
const (
	divZeroA = 10 / iota  // division by zero when iota=0
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("divZero", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	// should handle division by zero gracefully
	assert.Equal(t, 0, gen.values["divZeroA"].value)
}

func TestInvalidUTF8CharacterLiteral(t *testing.T) {
	// test ConvertLiteralToInt with a hex value that is valid
	lit := &ast.BasicLit{
		Kind:  token.CHAR,
		Value: "'\\x80'", // this is handled correctly by strconv.Unquote
	}

	val, err := ConvertLiteralToInt(lit)
	require.Error(t, err) // should error because \x80 is not valid UTF-8 for a char
	assert.Contains(t, err.Error(), "invalid UTF-8")
	assert.Equal(t, 0, val)
}

func TestMultipleCharactersInLiteral(t *testing.T) {
	// test ConvertLiteralToInt with multiple characters
	lit := &ast.BasicLit{
		Kind:  token.CHAR,
		Value: "'ab'", // invalid: multiple characters
	}

	val, err := ConvertLiteralToInt(lit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse character literal")
	assert.Equal(t, 0, val)
}

func TestGenerateWriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type writeErr int
const (
	writeErrA = iota
	writeErrB
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("writeErr", "/nonexistent/path/that/cannot/be/created/because/parent/does/not/exist")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	err = gen.Generate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create output directory")
}

func TestEmptyValueSpec(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	// create a const block with type declaration but no names
	src := `package test
type emptySpec int
const (
	emptySpecA = iota
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("emptySpec", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 0, gen.values["emptySpecA"].value)
}

func TestProcessExplicitValueDefaultReturn(t *testing.T) {
	gen, err := New("test", "")
	require.NoError(t, err)

	state := &constParseState{}

	// test with an unsupported expression type to trigger default return
	expr := &ast.ParenExpr{} // unsupported type
	result := gen.processExplicitValue(expr, state)
	assert.Equal(t, 0, result)
}

func TestApplyIotaOperationDefaultCase(t *testing.T) {
	gen, err := New("test", "")
	require.NoError(t, err)

	// test with unsupported operation to trigger default case
	op := &iotaOperation{
		op:         token.AND, // unsupported operation
		operand:    5,
		iotaOnLeft: true,
	}

	result := gen.applyIotaOperation(op, 10)
	assert.Equal(t, 10, result) // should return iotaVal unchanged
}

func TestProcessBinaryExprError(t *testing.T) {
	gen, err := New("test", "")
	require.NoError(t, err)

	state := &constParseState{iotaVal: 5}

	// create an invalid binary expression
	expr := &ast.BinaryExpr{
		X:  &ast.FuncLit{}, // unsupported type
		Op: token.ADD,
		Y:  &ast.BasicLit{Kind: token.INT, Value: "10"},
	}

	val, op := gen.processBinaryExpr(expr, state)
	assert.Equal(t, 0, val)
	assert.Nil(t, op)
}

func TestRightSideDivisionByIota(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type divByIota int
const (
	divByIotaA = iota     // 0
	divByIotaB = 10 / iota  // 10/1 = 10
	divByIotaC              // 10/2 = 5
	divByIotaD              // 10/3 = 3
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("divByIota", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	assert.Equal(t, 0, gen.values["divByIotaA"].value)
	assert.Equal(t, 10, gen.values["divByIotaB"].value)
	assert.Equal(t, 5, gen.values["divByIotaC"].value)
	assert.Equal(t, 3, gen.values["divByIotaD"].value)
}

func TestMultipleCharactersError(t *testing.T) {
	// directly test the multiple characters check in ConvertLiteralToInt
	// we need to craft a value that passes strconv.Unquote but has multiple runes
	lit := &ast.BasicLit{
		Kind:  token.CHAR,
		Value: "'\\u0041\\u0042'", // 'AB' - two unicode characters
	}

	val, err := ConvertLiteralToInt(lit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "character literal")
	assert.Equal(t, 0, val)
}

func TestWriteFilePermissionError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type perm int
const (
	permA = iota
	permB
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	// create a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o755))

	gen, err := New("perm", readOnlyDir)
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	// make the directory read-only to cause write failure
	require.NoError(t, os.Chmod(readOnlyDir, 0o555))
	defer os.Chmod(readOnlyDir, 0o755) // restore permissions for cleanup

	err = gen.Generate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write output file")
}

func TestParseConstBlockWithImportSpec(t *testing.T) {
	// test that parseConstBlock handles non-ValueSpec entries correctly
	gen, err := New("test", "")
	require.NoError(t, err)
	gen.pkgName = "test"

	// create a GenDecl with an ImportSpec (not a ValueSpec)
	decl := &ast.GenDecl{
		Tok: token.CONST,
		Specs: []ast.Spec{
			&ast.ImportSpec{}, // this should be skipped
		},
	}

	// this should not panic and should handle gracefully
	gen.parseConstBlock(decl)

	// no values should be added
	assert.Empty(t, gen.values)
}

func TestApplyIotaOperationDivisionByZeroRightSide(t *testing.T) {
	gen, err := New("test", "")
	require.NoError(t, err)

	// test division when iota is 0 and iota is on the right side
	op := &iotaOperation{
		op:         token.QUO,
		operand:    10,
		iotaOnLeft: false, // operand / iota
	}

	// when iota is 0, division by zero should return 0
	result := gen.applyIotaOperation(op, 0)
	assert.Equal(t, 0, result)
}

func TestConvertLiteralToIntMultipleRunes(t *testing.T) {
	// test the case where strconv.Unquote returns an error
	lit := &ast.BasicLit{
		Kind:  token.CHAR,
		Value: "'\\U00010000\\U00010001'", // invalid: two unicode code points
	}

	val, err := ConvertLiteralToInt(lit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot parse character literal")
	assert.Equal(t, 0, val)
}
