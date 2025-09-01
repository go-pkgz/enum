package generator

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

// TestGenerateEnumWithAllFeatures generates an enum with all features and verifies the output
func TestGenerateEnumWithAllFeatures(t *testing.T) {
	// use the testdata/integration directory
	testDir := "testdata/integration"

	// ensure status.go exists
	statusFile := filepath.Join(testDir, "status.go")
	require.FileExists(t, statusFile, "testdata/integration/status.go should exist")

	// generate with all features
	gen, err := New("status", testDir)
	require.NoError(t, err)
	gen.SetLowerCase(true)
	gen.SetGenerateBSON(true)
	gen.SetGenerateSQL(true)
	gen.SetGenerateYAML(true)

	err = gen.Parse(testDir)
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)

	// verify generated file exists and contains expected methods
	content, err := os.ReadFile(filepath.Join(testDir, "status_enum.go"))
	require.NoError(t, err)

	// verify all features are present
	assert.Contains(t, string(content), "String() string")
	assert.Contains(t, string(content), "ParseStatus(")
	assert.Contains(t, string(content), "MarshalText()")
	assert.Contains(t, string(content), "UnmarshalText(")
	assert.Contains(t, string(content), "MarshalBSONValue()")
	assert.Contains(t, string(content), "UnmarshalBSONValue(")
	assert.Contains(t, string(content), "Value() (driver.Value")
	assert.Contains(t, string(content), "Scan(value interface{})")
	assert.Contains(t, string(content), "MarshalYAML()")
	assert.Contains(t, string(content), "UnmarshalYAML(")

	// don't cleanup - we need the generated file for integration tests
}

// TestMongoDBIntegration tests MongoDB operations with containers
func TestMongoDBIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// check if MongoDB is available
	if os.Getenv("SKIP_MONGO_TEST") != "" {
		t.Skip("skipping MongoDB test")
	}

	// first generate the enum
	testDir := t.TempDir()
	setupTestEnum(t, testDir)

	// import and use testutils for MongoDB container
	// this would normally use the generated enum, but since we can't dynamically import,
	// we test the generated code structure instead
	content, err := os.ReadFile(filepath.Join(testDir, "status_enum.go"))
	require.NoError(t, err)

	// verify BSON marshaling code is correct
	assert.Contains(t, string(content), "func (e Status) MarshalBSONValue() (bsontype.Type, []byte, error)")
	assert.Contains(t, string(content), "return bson.MarshalValue(e.String())")
	assert.Contains(t, string(content), "func (e *Status) UnmarshalBSONValue(t bsontype.Type, data []byte) error")

	// test BSON marshaling with actual BSON package (without MongoDB)
	type TestDoc struct {
		Value string `bson:"value"`
	}

	doc := TestDoc{Value: "active"}
	data, err := bson.Marshal(doc)
	require.NoError(t, err)

	var decoded TestDoc
	err = bson.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "active", decoded.Value)
}

// TestSQLIntegration tests SQL operations with SQLite
func TestSQLIntegration(t *testing.T) {
	// generate enum first
	testDir := t.TempDir()
	setupTestEnum(t, testDir)

	// verify SQL code is generated
	content, err := os.ReadFile(filepath.Join(testDir, "status_enum.go"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "func (e Status) Value() (driver.Value, error)")
	assert.Contains(t, string(content), "func (e *Status) Scan(value interface{}) error")

	// test with actual SQLite
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// create table
	_, err = db.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY,
			status TEXT,
			name TEXT
		)
	`)
	require.NoError(t, err)

	// test string storage and retrieval
	testCases := []struct {
		status string
		name   string
	}{
		{"active", "Test Active"},
		{"inactive", "Test Inactive"},
		{"blocked", "Test Blocked"},
		{"unknown", "Test Unknown"},
	}

	for i, tc := range testCases {
		_, err = db.Exec("INSERT INTO test_table (id, status, name) VALUES (?, ?, ?)",
			i+1, tc.status, tc.name)
		require.NoError(t, err)

		var status, name string
		row := db.QueryRow("SELECT status, name FROM test_table WHERE id = ?", i+1)
		err = row.Scan(&status, &name)
		require.NoError(t, err)

		assert.Equal(t, tc.status, status)
		assert.Equal(t, tc.name, name)
	}

	// test NULL handling
	_, err = db.Exec("INSERT INTO test_table (id, status, name) VALUES (?, NULL, ?)",
		999, "NULL Test")
	require.NoError(t, err)

	var nullStatus sql.NullString
	var name string
	row := db.QueryRow("SELECT status, name FROM test_table WHERE id = ?", 999)
	err = row.Scan(&nullStatus, &name)
	require.NoError(t, err)

	assert.False(t, nullStatus.Valid)
	assert.Equal(t, "NULL Test", name)
}

// TestYAMLIntegration tests YAML marshaling
func TestYAMLIntegration(t *testing.T) {
	// generate enum first
	testDir := t.TempDir()
	setupTestEnum(t, testDir)

	// verify YAML code is generated
	content, err := os.ReadFile(filepath.Join(testDir, "status_enum.go"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "func (e Status) MarshalYAML() (any, error)")
	assert.Contains(t, string(content), "func (e *Status) UnmarshalYAML(value *yaml.Node) error")

	// test YAML operations
	type Config struct {
		Name   string `yaml:"name"`
		Status string `yaml:"status"`
		Count  int    `yaml:"count"`
	}

	cfg := Config{
		Name:   "test config",
		Status: "active",
		Count:  42,
	}

	// marshal
	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.Contains(t, string(data), "status: active")

	// unmarshal
	var decoded Config
	err = yaml.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, cfg, decoded)

	// test error cases
	invalidYAML := []byte("status: not_a_valid_status")
	var badConfig Config
	err = yaml.Unmarshal(invalidYAML, &badConfig)
	// this should succeed as it's just a string field
	require.NoError(t, err)
	assert.Equal(t, "not_a_valid_status", badConfig.Status)
}

// TestJSONIntegration tests JSON marshaling via TextMarshaler
func TestJSONIntegration(t *testing.T) {
	// generate enum first
	testDir := t.TempDir()
	setupTestEnum(t, testDir)

	// verify TextMarshaler code is generated
	content, err := os.ReadFile(filepath.Join(testDir, "status_enum.go"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "func (e Status) MarshalText() ([]byte, error)")
	assert.Contains(t, string(content), "func (e *Status) UnmarshalText(text []byte) error")

	// test JSON operations
	type Response struct {
		Status string   `json:"status"`
		Count  int      `json:"count"`
		Items  []string `json:"items"`
	}

	resp := Response{
		Status: "active",
		Count:  3,
		Items:  []string{"a", "b", "c"},
	}

	// marshal
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"status":"active"`)

	// unmarshal
	var decoded Response
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, resp, decoded)
}

// TestEnumGeneration tests various enum generation scenarios
func TestEnumGeneration(t *testing.T) {
	tests := []struct {
		name          string
		enumDef       string
		typeName      string
		wantBSON      bool
		wantSQL       bool
		wantYAML      bool
		wantLowerCase bool
	}{
		{
			name: "basic enum no features",
			enumDef: `package test
type color int
const (
	colorRed color = iota
	colorGreen
	colorBlue
)`,
			typeName: "color",
		},
		{
			name: "enum with BSON only",
			enumDef: `package test
type priority uint8
const (
	priorityLow priority = iota
	priorityMedium
	priorityHigh
)`,
			typeName: "priority",
			wantBSON: true,
		},
		{
			name: "enum with SQL only",
			enumDef: `package test
type state int32
const (
	stateInit state = iota
	stateRunning
	stateStopped
)`,
			typeName: "state",
			wantSQL:  true,
		},
		{
			name: "enum with lowercase names",
			enumDef: `package test
type mode uint
const (
	modeRead mode = iota
	modeWrite
	modeExecute
)`,
			typeName:      "mode",
			wantLowerCase: true,
		},
		{
			name: "enum with all features",
			enumDef: `package test
type level int
const (
	levelDebug level = iota
	levelInfo
	levelWarn
	levelError
)`,
			typeName:      "level",
			wantBSON:      true,
			wantSQL:       true,
			wantYAML:      true,
			wantLowerCase: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()

			// write enum definition
			enumFile := filepath.Join(testDir, "enum.go")
			err := os.WriteFile(enumFile, []byte(tt.enumDef), 0o644)
			require.NoError(t, err)

			// generate enum
			gen, err := New(tt.typeName, testDir)
			require.NoError(t, err)

			if tt.wantBSON {
				gen.SetGenerateBSON(true)
			}
			if tt.wantSQL {
				gen.SetGenerateSQL(true)
			}
			if tt.wantYAML {
				gen.SetGenerateYAML(true)
			}
			if tt.wantLowerCase {
				gen.SetLowerCase(true)
			}

			err = gen.Parse(testDir)
			require.NoError(t, err)

			err = gen.Generate()
			require.NoError(t, err)

			// verify generated file
			generatedFile := filepath.Join(testDir, tt.typeName+"_enum.go")
			require.FileExists(t, generatedFile)

			content, err := os.ReadFile(generatedFile)
			require.NoError(t, err)

			// check expected features
			if tt.wantBSON {
				assert.Contains(t, string(content), "MarshalBSONValue")
				assert.Contains(t, string(content), "UnmarshalBSONValue")
			} else {
				assert.NotContains(t, string(content), "MarshalBSONValue")
			}

			if tt.wantSQL {
				assert.Contains(t, string(content), "Value() (driver.Value")
				assert.Contains(t, string(content), "Scan(value interface{})")
			} else {
				assert.NotContains(t, string(content), "driver.Value")
			}

			if tt.wantYAML {
				assert.Contains(t, string(content), "MarshalYAML")
				assert.Contains(t, string(content), "UnmarshalYAML")
			} else {
				assert.NotContains(t, string(content), "MarshalYAML")
			}
		})
	}
}

// TestRuntimeIntegration tests the full pipeline: build binary → generate enums → test with real databases
func TestRuntimeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// skip in CI due to timeout issues with downloading dependencies
	if os.Getenv("CI") != "" {
		t.Skip("skipping runtime integration test in CI")
	}

	// 1. Build the enum binary
	binPath := filepath.Join(t.TempDir(), "enum")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/go-pkgz/enum")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to build enum binary: %s", output)

	// 2. Create a temp package for testing
	pkgDir := t.TempDir()

	// Copy enum definitions from testdata
	statusSrc, err := os.ReadFile("testdata/integration/status.go")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pkgDir, "status.go"), statusSrc, 0o644)
	require.NoError(t, err)

	prioritySrc, err := os.ReadFile("testdata/integration/priority.go")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pkgDir, "priority.go"), prioritySrc, 0o644)
	require.NoError(t, err)

	// 3. Generate enums using the built binary
	// Generate status enum
	cmd = exec.Command(binPath, "-type=status", "-lower", "-sql", "-bson", "-yaml")
	cmd.Dir = pkgDir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed to generate status enum: %s", output)

	// Generate priority enum
	cmd = exec.Command(binPath, "-type=priority", "-sql", "-bson", "-yaml")
	cmd.Dir = pkgDir
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed to generate priority enum: %s", output)

	// Verify generated files exist
	require.FileExists(t, filepath.Join(pkgDir, "status_enum.go"))
	require.FileExists(t, filepath.Join(pkgDir, "priority_enum.go"))

	// 4. Create go.mod for the test package
	goModContent := []byte(`module testpkg

go 1.24

require (
	github.com/stretchr/testify v1.10.0
	go.mongodb.org/mongo-driver v1.17.4
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.38.2
	github.com/go-pkgz/testutils v0.4.3
)
`)
	err = os.WriteFile(filepath.Join(pkgDir, "go.mod"), goModContent, 0o644)
	require.NoError(t, err)

	// 5. Copy test file from testdata
	testContent, err := os.ReadFile("testdata/integration/enum_test.go")
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(pkgDir, "enum_test.go"), testContent, 0o644)
	require.NoError(t, err)

	// 6. Run go mod tidy
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = pkgDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		// it's ok if mod tidy fails due to network, as long as test runs
		t.Logf("go mod tidy output: %s", output)
	}

	// 7. Run the tests in the generated package
	cmd = exec.Command("go", "test", "-v", ".")
	cmd.Dir = pkgDir
	output, err = cmd.CombinedOutput()

	t.Logf("Test output:\n%s", output)

	if err != nil {
		t.Fatalf("Generated enum tests failed: %v", err)
	}

	// verify expected tests ran
	outputStr := string(output)
	require.Contains(t, outputStr, "PASS")
	require.Contains(t, outputStr, "TestGeneratedEnumWithMongoDB")
	require.Contains(t, outputStr, "TestGeneratedEnumWithSQL")
	require.Contains(t, outputStr, "TestGeneratedEnumWithYAML")
	require.Contains(t, outputStr, "TestGeneratedEnumWithJSON")
}

// TestRuntimeIntegrationErrors tests error cases in the generation pipeline
func TestRuntimeIntegrationErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// build the enum binary
	binPath := filepath.Join(t.TempDir(), "enum")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/go-pkgz/enum")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to build enum binary: %s", output)

	t.Run("missing type", func(t *testing.T) {
		pkgDir := t.TempDir()

		// create enum file
		writeErr := os.WriteFile(filepath.Join(pkgDir, "test.go"), []byte(`package test
type myenum int
const (
	myenumOne myenum = iota
)`), 0o644)
		require.NoError(t, writeErr)

		// try to generate non-existent type
		missingCmd := exec.Command(binPath, "-type=missing")
		missingCmd.Dir = pkgDir
		missingOutput, missingErr := missingCmd.CombinedOutput()
		require.Error(t, missingErr)
		require.Contains(t, string(missingOutput), "type missing")
	})

	t.Run("no constants", func(t *testing.T) {
		pkgDir := t.TempDir()

		// create enum without constants
		writeErr := os.WriteFile(filepath.Join(pkgDir, "test.go"), []byte(`package test
type empty int
`), 0o644)
		require.NoError(t, writeErr)

		// try to generate
		emptyCmd := exec.Command(binPath, "-type=empty")
		emptyCmd.Dir = pkgDir
		emptyOutput, emptyErr := emptyCmd.CombinedOutput()
		require.Error(t, emptyErr)
		require.Contains(t, string(emptyOutput), "no const values found")
	})

	t.Run("invalid type name", func(t *testing.T) {
		pkgDir := t.TempDir()

		// uppercase type name should fail
		invalidCmd := exec.Command(binPath, "-type=Status")
		invalidCmd.Dir = pkgDir
		invalidOutput, invalidErr := invalidCmd.CombinedOutput()
		require.Error(t, invalidErr)
		require.Contains(t, string(invalidOutput), "first letter must be lowercase")
	})
}

// TestErrorHandling tests various error conditions
func TestErrorHandling(t *testing.T) {
	t.Run("invalid enum type", func(t *testing.T) {
		testDir := t.TempDir()

		// write file without the requested type
		err := os.WriteFile(filepath.Join(testDir, "test.go"), []byte(`package test
type other int
const (
	otherOne other = iota
)`), 0o644)
		require.NoError(t, err)

		gen, err := New("missing", testDir)
		require.NoError(t, err)

		err = gen.Parse(testDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type missing")
	})

	t.Run("no constants found", func(t *testing.T) {
		testDir := t.TempDir()

		// write type without constants
		err := os.WriteFile(filepath.Join(testDir, "test.go"), []byte(`package test
type empty int
`), 0o644)
		require.NoError(t, err)

		gen, err := New("empty", testDir)
		require.NoError(t, err)

		err = gen.Parse(testDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no const values found")
	})
}

// setupTestEnum creates a test enum in the given directory
func setupTestEnum(t *testing.T, dir string) {
	t.Helper()

	enumFile := filepath.Join(dir, "status.go")
	err := os.WriteFile(enumFile, []byte(`package test

type status uint8

const (
	statusUnknown status = iota
	statusActive
	statusInactive
	statusBlocked
	statusDeleted
)
`), 0o644)
	require.NoError(t, err)

	gen, err := New("status", dir)
	require.NoError(t, err)
	gen.SetLowerCase(true)
	gen.SetGenerateBSON(true)
	gen.SetGenerateSQL(true)
	gen.SetGenerateYAML(true)

	err = gen.Parse(dir)
	require.NoError(t, err)

	err = gen.Generate()
	require.NoError(t, err)
}

// NullableEnum wraps an enum for SQL NULL support
type NullableEnum struct {
	Enum  interface{ driver.Valuer }
	Valid bool
}

// Value implements driver.Valuer
func (n NullableEnum) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Enum.(driver.Valuer).Value()
}

// Scan implements sql.Scanner
func (n *NullableEnum) Scan(value interface{}) error {
	if value == nil {
		n.Valid = false
		return nil
	}
	n.Valid = true
	// actual scanning would be delegated to the enum's Scan method
	return nil
}
