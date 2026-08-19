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

		// enable SQL to verify SQL-specific imports/methods when requested
		gen.SetGenerateSQL(true)

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

		gen.SetGenerateSQL(true)
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

	t.Run("bson support", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		gen.SetGenerateBSON(true)
		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)

		// verify bson interfaces and imports
		assert.Contains(t, string(content), `"go.mongodb.org/mongo-driver/bson"`)
		assert.Contains(t, string(content), `"go.mongodb.org/mongo-driver/bson/bsontype"`)
		assert.Contains(t, string(content), "func (e Status) MarshalBSONValue() (bsontype.Type, []byte, error)")
		assert.Contains(t, string(content), "func (e *Status) UnmarshalBSONValue(t bsontype.Type, data []byte) error")
	})

	t.Run("yaml support", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		gen.SetGenerateYAML(true)
		err = gen.Generate()
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)

		// verify yaml interfaces and imports
		assert.Contains(t, string(content), `"gopkg.in/yaml.v3"`)
		assert.Contains(t, string(content), "func (e Status) MarshalYAML() (any, error)")
		assert.Contains(t, string(content), "func (e *Status) UnmarshalYAML(value *yaml.Node) error")
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

		// verify UnmarshalText uses Parse
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

	assert.Equal(t, int64(0), constVal(t, gen, "statusUnknown"), "unknown should be 0")
	assert.Equal(t, int64(1), constVal(t, gen, "statusActive"), "active should be 1")
	assert.Equal(t, int64(2), constVal(t, gen, "statusInactive"), "inactive should be 2")
	assert.Equal(t, int64(3), constVal(t, gen, "statusBlocked"), "blocked should be 3")
}

func TestRepeatValues(t *testing.T) {
	tmpDir := t.TempDir()

	gen, err := New("repeatValues", tmpDir)
	require.NoError(t, err)

	err = gen.Parse("testdata")
	require.NoError(t, err)

	assert.Equal(t, int64(10), constVal(t, gen, "repeatValuesFirst"), "First should be 10")
	assert.Equal(t, int64(10), constVal(t, gen, "repeatValuesSecond"), "Second should repeat the value 10")
	assert.Equal(t, int64(20), constVal(t, gen, "repeatValuesThird"), "Third should be 20")
	assert.Equal(t, int64(20), constVal(t, gen, "repeatValuesFourth"), "Fourth should repeat the value 20")
}

func TestSQLNullHandling(t *testing.T) {
	t.Run("with zero value", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		gen.SetGenerateSQL(true)
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

		gen.SetGenerateSQL(true)
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
	assert.Equal(t, int64(1), constVal(t, gen, "binaryExprFirst"), "First should be 1")
	assert.Equal(t, int64(2), constVal(t, gen, "binaryExprSecond"), "Second should be 2")
	assert.Equal(t, int64(3), constVal(t, gen, "binaryExprThird"), "Third should be 3")

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
		// parsing is always case-insensitive, so strings.ToLower is always used
		parseIdx := bytes.Index(content, []byte("func ParseStatus"))
		parseEnd := bytes.Index(content[parseIdx:], []byte("}"))
		parseFunc := string(content[parseIdx : parseIdx+parseEnd])
		assert.Contains(t, parseFunc, "strings.ToLower")
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

func TestNegativeEnumValues(t *testing.T) {
	t.Run("negative integers", func(t *testing.T) {
		tmpDir := t.TempDir()

		// create enum with negative values
		enumFile := filepath.Join(tmpDir, "test.go")
		err := os.WriteFile(enumFile, []byte(`package test

type errorCode int

const (
	errorCodeNone    errorCode = -1
	errorCodeOK      errorCode = 0
	errorCodeBadRequest errorCode = 400
	errorCodeNotFound   errorCode = 404
)`), 0o644)
		require.NoError(t, err)

		gen, err := New("errorCode", tmpDir)
		require.NoError(t, err)

		err = gen.Parse(tmpDir)
		require.NoError(t, err)

		// verify negative value was parsed correctly
		assert.Equal(t, int64(-1), constVal(t, gen, "errorCodeNone"))
		assert.Equal(t, int64(0), constVal(t, gen, "errorCodeOK"))
		assert.Equal(t, int64(400), constVal(t, gen, "errorCodeBadRequest"))
		assert.Equal(t, int64(404), constVal(t, gen, "errorCodeNotFound"))

		err = gen.Generate()
		require.NoError(t, err)

		// verify generated code has correct values
		content, err := os.ReadFile(filepath.Join(tmpDir, "error_code_enum.go"))
		require.NoError(t, err)

		contentStr := string(content)
		assert.Contains(t, contentStr, "ErrorCodeNone       = ErrorCode{name: \"None\", value: -1}")
		assert.Contains(t, contentStr, "ErrorCodeOK         = ErrorCode{name: \"OK\", value: 0}")
		assert.Contains(t, contentStr, "ErrorCodeBadRequest = ErrorCode{name: \"BadRequest\", value: 400}")
		assert.Contains(t, contentStr, "ErrorCodeNotFound   = ErrorCode{name: \"NotFound\", value: 404}")
	})

	t.Run("invalid negative expression", func(t *testing.T) {
		tmpDir := t.TempDir()

		// create enum with an expression that has no integer value
		enumFile := filepath.Join(tmpDir, "test.go")
		err := os.WriteFile(enumFile, []byte(`package test

type status int

const (
	statusInvalid status = -"not_a_number"
	statusOK      status = 1
)`), 0o644)
		require.NoError(t, err)

		gen, err := New("status", tmpDir)
		require.NoError(t, err)

		err = gen.Parse(tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to evaluate value of const statusInvalid")
	})
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
		assert.Equal(t, int64(0), constVal(t, gen, "mulDivTypeA"))
		assert.Equal(t, int64(2), constVal(t, gen, "mulDivTypeB"))
		assert.Equal(t, int64(4), constVal(t, gen, "mulDivTypeC"))
	})

	t.Run("right-side iota addition", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("rightIotaType", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// check values
		assert.Equal(t, int64(10), constVal(t, gen, "rightIotaTypeX"))
		assert.Equal(t, int64(11), constVal(t, gen, "rightIotaTypeY"))
	})

	t.Run("subtraction with iota", func(t *testing.T) {
		tmpDir := t.TempDir()
		gen, err := New("subType", tmpDir)
		require.NoError(t, err)

		err = gen.Parse("testdata")
		require.NoError(t, err)

		// check values
		assert.Equal(t, int64(100), constVal(t, gen, "subTypeA"))
		assert.Equal(t, int64(99), constVal(t, gen, "subTypeB"))
		assert.Equal(t, int64(98), constVal(t, gen, "subTypeC"))
	})
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
	assert.Equal(t, int64(0), constVal(t, gen, "statusFirst"))
	assert.Equal(t, int64(2), constVal(t, gen, "statusSecond")) // iota=2 (after _ at iota=1)
	assert.Equal(t, int64(4), constVal(t, gen, "statusThird"))  // iota=4 (after _ at iota=3)
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
	assert.Equal(t, int64(0), constVal(t, gen, "divTypeA"))
	assert.Equal(t, int64(0), constVal(t, gen, "divTypeB"))
	assert.Equal(t, int64(1), constVal(t, gen, "divTypeC"))
	assert.Equal(t, int64(1), constVal(t, gen, "divTypeD"))
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

	assert.Equal(t, int64(10), constVal(t, gen, "subTypeA"))
	assert.Equal(t, int64(9), constVal(t, gen, "subTypeB"))
	assert.Equal(t, int64(8), constVal(t, gen, "subTypeC"))
	assert.Equal(t, int64(2), constVal(t, gen, "subTypeD"))
	assert.Equal(t, int64(3), constVal(t, gen, "subTypeE"))
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

	assert.Equal(t, int64(0), constVal(t, gen, "emptyTypeFirst"))
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

	assert.Equal(t, int64(0), constVal(t, gen, "zeroTypeA"))
	assert.Equal(t, int64(1), constVal(t, gen, "zeroTypeB"))
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "division by zero")
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

	assert.Equal(t, int64(0), constVal(t, gen, "emptySpecA"))
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

	assert.Equal(t, int64(0), constVal(t, gen, "divByIotaA"))
	assert.Equal(t, int64(10), constVal(t, gen, "divByIotaB"))
	assert.Equal(t, int64(5), constVal(t, gen, "divByIotaC"))
	assert.Equal(t, int64(3), constVal(t, gen, "divByIotaD"))
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
	require.NoError(t, gen.parseConstBlock(decl, newConstResolver()))

	// no values should be added
	assert.Empty(t, gen.values)
}

func TestParseAliasComment(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		expected []string
	}{
		{"basic alias", "// enum:alias=rw", []string{"rw"}},
		{"multiple aliases", "// enum:alias=rw,read-write", []string{"rw", "read-write"}},
		{"with whitespace", "// enum:alias= rw , read-write ", []string{"rw", "read-write"}},
		{"empty value", "// enum:alias=", nil},
		{"only separators", "// enum:alias=,,", nil},
		{"empty between commas", "// enum:alias=a,,b", []string{"a", "b"}},
		{"no alias directive", "// some comment", nil},
		{"nil comment", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var comment *ast.CommentGroup
			if tt.comment != "" {
				comment = &ast.CommentGroup{
					List: []*ast.Comment{{Text: tt.comment}},
				}
			}
			result := parseAliasComment(comment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDocComment(t *testing.T) {
	tests := []struct {
		name     string
		comments []string // each string is one // comment line
		expected string
	}{
		{
			name:     "nil comment group",
			comments: nil,
			expected: "",
		},
		{
			name:     "basic inline description",
			comments: []string{"// Should be first"},
			expected: "Should be first",
		},
		{
			name:     "strips leading space after //",
			comments: []string{"//  Should be first"},
			expected: "Should be first",
		},
		{
			name:     "description with parentheses",
			comments: []string{"// Should be first (alphabetically would be fourth)"},
			expected: "Should be first (alphabetically would be fourth)",
		},
		{
			name:     "enum directive only — returns empty",
			comments: []string{"// enum:alias=rw"},
			expected: "",
		},
		{
			name:     "empty comment text — returns empty",
			comments: []string{"//"},
			expected: "",
		},
		{
			name:     "multi-line description joined with space",
			comments: []string{"// First line", "// second line"},
			expected: "First line second line",
		},
		{
			name:     "directive line filtered, text line kept",
			comments: []string{"// My description", "// enum:alias=a,b"},
			expected: "My description",
		},
		{
			name:     "directive line first, text line kept",
			comments: []string{"// enum:alias=rw", "// Read-Write access level"},
			expected: "Read-Write access level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var comment *ast.CommentGroup
			if tt.comments != nil {
				list := make([]*ast.Comment, len(tt.comments))
				for i, c := range tt.comments {
					list[i] = &ast.Comment{Text: c}
				}
				comment = &ast.CommentGroup{List: list}
			}
			result := parseDocComment(comment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseWithAliases(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type status int
const (
	statusActive status = iota // enum:alias=a,on
	statusInactive             // enum:alias=i,off
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("status", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	// verify aliases are extracted
	assert.Equal(t, []string{"a", "on"}, gen.values["statusActive"].aliases)
	assert.Equal(t, []string{"i", "off"}, gen.values["statusInactive"].aliases)
}

func TestParseWithoutAliases(t *testing.T) {
	// ensure backward compatibility - constants without aliases should have nil aliases
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type status int
const (
	statusActive status = iota
	statusInactive
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("status", "")
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	// verify no aliases
	assert.Nil(t, gen.values["statusActive"].aliases)
	assert.Nil(t, gen.values["statusInactive"].aliases)
}

func TestGenerateWithDuplicateAliases(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type status int
const (
	statusA status = iota // enum:alias=x
	statusB               // enum:alias=x
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("status", tmpDir)
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	err = gen.Generate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate alias")
	assert.Contains(t, err.Error(), "x")
}

func TestGenerateWithCanonicalConflict(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type status int
const (
	statusActive status = iota // enum:alias=inactive
	statusInactive
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("status", tmpDir)
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	err = gen.Generate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicts with canonical")
}

func TestGenerateAliasesInParseMap(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type status int
const (
	statusActive status = iota // enum:alias=a,on
	statusInactive             // enum:alias=i,off
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("status", tmpDir)
	require.NoError(t, err)
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
	require.NoError(t, err)

	// verify parse map contains canonical names
	assert.Contains(t, string(content), `"active":`)
	assert.Contains(t, string(content), `"inactive":`)

	// verify parse map contains aliases
	assert.Contains(t, string(content), `"a":`)
	assert.Contains(t, string(content), `"on":`)
	assert.Contains(t, string(content), `"i":`)
	assert.Contains(t, string(content), `"off":`)

	// verify aliases point to correct enum values
	assert.Contains(t, string(content), `"a":        StatusActive`)
	assert.Contains(t, string(content), `"on":       StatusActive`)
	assert.Contains(t, string(content), `"i":        StatusInactive`)
	assert.Contains(t, string(content), `"off":      StatusInactive`)
}

func TestGenerateConstComments(t *testing.T) {
	tmpDir := t.TempDir()

	src := `package testpkg

type orderTest uint8

const (
	orderTestZero    orderTest = iota // Should be first (alphabetically would be fourth)
	orderTestAlpha                    // Should be second (alphabetically would be first)
	orderTestCharlie                  // Should be third (alphabetically would be third)
	orderTestBravo                    // Should be fourth (alphabetically would be second)
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "order.go"), []byte(src), 0o600)
	require.NoError(t, err)

	gen, err := New("orderTest", tmpDir)
	require.NoError(t, err)
	require.NoError(t, gen.Parse(tmpDir))
	require.NoError(t, gen.Generate())

	content, err := os.ReadFile(filepath.Join(tmpDir, "order_test_enum.go"))
	require.NoError(t, err)
	out := string(content)

	// block comment still present
	assert.Contains(t, out, "// Public constants for orderTest values")

	// individual comments present
	assert.Contains(t, out, "// Should be first (alphabetically would be fourth)")
	assert.Contains(t, out, "// Should be second (alphabetically would be first)")
	assert.Contains(t, out, "// Should be third (alphabetically would be third)")
	assert.Contains(t, out, "// Should be fourth (alphabetically would be second)")

	// each comment appears before its constant (check relative positions)
	zeroCommentPos := strings.Index(out, "// Should be first (alphabetically would be fourth)")
	zeroConstPos := strings.Index(out, "OrderTestZero =")
	assert.Less(t, zeroCommentPos, zeroConstPos, "comment should appear before constant")

	alphaCommentPos := strings.Index(out, "// Should be second (alphabetically would be first)")
	alphaConstPos := strings.Index(out, "OrderTestAlpha =")
	assert.Less(t, alphaCommentPos, alphaConstPos, "comment should appear before constant")
}

func TestGenerateConstCommentsDocAboveFallback(t *testing.T) {
	tmpDir := t.TempDir()

	src := `package testpkg

type orderTest uint8

const (
	// The zero value — default when unset
	orderTestZero orderTest = iota
	// Alphabetically first but declared second
	orderTestAlpha
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "order.go"), []byte(src), 0o600)
	require.NoError(t, err)

	gen, err := New("orderTest", tmpDir)
	require.NoError(t, err)
	require.NoError(t, gen.Parse(tmpDir))
	require.NoError(t, gen.Generate())

	content, err := os.ReadFile(filepath.Join(tmpDir, "order_test_enum.go"))
	require.NoError(t, err)
	out := string(content)

	assert.Contains(t, out, "// The zero value — default when unset")
	assert.Contains(t, out, "// Alphabetically first but declared second")
}

func TestGenerateConstCommentsInlineOverridesDoc(t *testing.T) {
	tmpDir := t.TempDir()

	src := `package testpkg

type orderTest uint8

const (
	// doc comment — should NOT appear
	orderTestZero orderTest = iota // inline comment wins
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "order.go"), []byte(src), 0o600)
	require.NoError(t, err)

	gen, err := New("orderTest", tmpDir)
	require.NoError(t, err)
	require.NoError(t, gen.Parse(tmpDir))
	require.NoError(t, gen.Generate())

	content, err := os.ReadFile(filepath.Join(tmpDir, "order_test_enum.go"))
	require.NoError(t, err)
	out := string(content)

	assert.Contains(t, out, "// inline comment wins")
	assert.NotContains(t, out, "// doc comment")
}

func TestGenerateNoCommentNoDiff(t *testing.T) {
	tmpDir := t.TempDir()

	src := `package testpkg

type status uint8

const (
	statusActive   status = iota
	statusInactive
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "status.go"), []byte(src), 0o600)
	require.NoError(t, err)

	gen, err := New("status", tmpDir)
	require.NoError(t, err)
	require.NoError(t, gen.Parse(tmpDir))
	require.NoError(t, gen.Generate())

	content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
	require.NoError(t, err)
	out := string(content)

	// no individual doc comment lines in the var block
	varBlockStart := strings.Index(out, "// Public constants for status values")
	varBlockEnd := strings.Index(out, "// StatusValues contains")
	varBlock := out[varBlockStart:varBlockEnd]
	assert.NotContains(t, varBlock, "//\n", "var block should have no stray empty comment lines")
	assert.NotContains(t, varBlock, "\t//\n", "var block should have no stray empty comment lines")
	assert.Contains(t, out, "StatusActive")
	assert.Contains(t, out, "StatusInactive")
}

func TestGenerateAliasAndCommentCoexist(t *testing.T) {
	tmpDir := t.TempDir()

	src := `package testpkg

type status uint8

const (
	statusReadWrite status = iota // enum:alias=rw,read-write
	// Administrator access
	statusAdmin // enum:alias=adm
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "status.go"), []byte(src), 0o600)
	require.NoError(t, err)

	gen, err := New("status", tmpDir)
	require.NoError(t, err)
	require.NoError(t, gen.Parse(tmpDir))
	require.NoError(t, gen.Generate())

	content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
	require.NoError(t, err)
	out := string(content)

	// aliases still work
	assert.Contains(t, out, `"rw":`)
	assert.Contains(t, out, `"read-write":`)
	assert.Contains(t, out, `"adm":`)

	// statusAdmin has doc comment
	assert.Contains(t, out, "// Administrator access")

	// no doc comment line before StatusReadWrite (directive-only inline → no comment emitted)
	assert.NotContains(t, out, "//\n\tStatusReadWrite =")
}

func TestGenerateLowerCaseWithMixedCaseAlias(t *testing.T) {
	// test that mixed-case aliases work with -lower flag
	// this was a bug: parse map keys are always lowercase but Parse() didn't normalize input
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package test
type permission int
const (
	permissionNone      permission = iota // enum:alias=n
	permissionReadWrite                   // enum:alias=RW,read-write
)
`
	require.NoError(t, os.WriteFile(testFile, []byte(src), 0o644))

	gen, err := New("permission", tmpDir)
	require.NoError(t, err)
	gen.SetLowerCase(true) // enable -lower flag
	err = gen.Parse(tmpDir)
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "permission_enum.go"))
	require.NoError(t, err)

	// verify generated parse function uses strings.ToLower
	assert.Contains(t, string(content), "strings.ToLower(v)")

	// verify aliases are stored lowercase in map
	assert.Contains(t, string(content), `"rw":`)
	assert.Contains(t, string(content), `"read-write":`)

	// verify parse function will handle mixed case input correctly by checking the template output
	// the parse function should always use strings.ToLower(v) for lookup
	assert.Contains(t, string(content), `_permissionParseMap[strings.ToLower(v)]`)
}
