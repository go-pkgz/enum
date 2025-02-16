package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// save original os.Exit and restore after test
var origExit = osExit

func TestMain(m *testing.M) {
	// remove all mocks after all tests
	defer func() { osExit = origExit }()
	m.Run()
}

func TestIntegration(t *testing.T) {
	// Reset flags between runs to avoid "flag redefined" error
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	t.Run("generate enum", func(t *testing.T) {
		// save original args and restore after test
		origArgs := os.Args
		origWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			os.Args = origArgs
			require.NoError(t, os.Chdir(origWd))
		}()

		tmpDir := t.TempDir()

		// copy testdata to tmp
		err = os.WriteFile(filepath.Join(tmpDir, "status.go"), []byte(`
package test
type status uint8
const (
	statusUnknown status = iota
	statusActive
	statusInactive
)
`), 0o644)
		require.NoError(t, err)

		// change working directory to temp dir
		require.NoError(t, os.Chdir(tmpDir))

		// no exit should happen here
		var exitCode int
		osExit = func(code int) { exitCode = code }

		// set args and run main
		os.Args = []string{"app", "-type", "status"}
		main()

		assert.Equal(t, 0, exitCode, "unexpected os.Exit call")

		// verify generated file
		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "type Status struct")
		assert.Contains(t, string(content), "StatusActive")
		assert.Contains(t, string(content), "StatusInactive")
	})

	t.Run("lower case", func(t *testing.T) {
		// Reset flags for this run
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		origArgs := os.Args
		origWd, err := os.Getwd()
		require.NoError(t, err)
		defer func() {
			os.Args = origArgs
			require.NoError(t, os.Chdir(origWd))
		}()

		tmpDir := t.TempDir()
		err = os.WriteFile(filepath.Join(tmpDir, "status.go"), []byte(`
package test
type status uint8
const (
	statusUnknown status = iota
	statusActive
)
`), 0o644)
		require.NoError(t, err)

		// change working directory to temp dir
		require.NoError(t, os.Chdir(tmpDir))

		var exitCode int
		osExit = func(code int) { exitCode = code }

		os.Args = []string{"app", "-type", "status", "-lower"}
		main()

		assert.Equal(t, 0, exitCode, "unexpected os.Exit call")

		content, err := os.ReadFile(filepath.Join(tmpDir, "status_enum.go"))
		require.NoError(t, err)
		assert.Contains(t, string(content), `name: "active"`)
	})

	t.Run("version", func(t *testing.T) {
		// Reset flags for this run
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		origArgs := os.Args
		defer func() { os.Args = origArgs }()

		var exitCode int
		osExit = func(code int) { exitCode = code }

		os.Args = []string{"app", "-version"}
		main()
		assert.Equal(t, 0, exitCode)
	})

	t.Run("help", func(t *testing.T) {
		// Reset flags for this run
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		origArgs := os.Args
		defer func() { os.Args = origArgs }()

		var exitCode int
		osExit = func(code int) { exitCode = code }

		os.Args = []string{"app", "-help"}
		main()
		assert.Equal(t, 0, exitCode)
	})

	t.Run("missing type", func(t *testing.T) {
		// Reset flags for this run
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		origArgs := os.Args
		defer func() { os.Args = origArgs }()

		var exitCode int
		osExit = func(code int) { exitCode = code }

		os.Args = []string{"app"}
		main()
		assert.Equal(t, 1, exitCode)
	})

	t.Run("uppercase type", func(t *testing.T) {
		// Reset flags for this run
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		origArgs := os.Args
		defer func() { os.Args = origArgs }()

		var exitCode int
		osExit = func(code int) { exitCode = code }

		os.Args = []string{"app", "-type", "Status"}
		main()
		assert.Equal(t, 1, exitCode)
	})
}
