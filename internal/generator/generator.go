// Package generator provides a code generator for enum types. It reads Go source files and extracts enum values
// to generate a new type with json, bson and text marshaling support.
package generator

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var titleCaser = cases.Title(language.English, cases.NoLower)

// Generator holds the data needed for enum code generation
type Generator struct {
	Type           string         // the private type name (e.g., "status")
	Path           string         // output directory path
	values         map[string]int // const values found
	pkgName        string         // package name from source file
	lowerCase      bool           // use lower case for marshal/unmarshal
	generateGetter bool           // generate getter methods for enum values
}

// Value represents a single enum value
type Value struct {
	PrivateName string // e.g., "statusActive"
	PublicName  string // e.g., "StatusActive"
	Name        string // e.g., "Active"
	Index       int    // enum index value
}

// New creates a new Generator instance
func New(typeName, path string) (*Generator, error) {
	if typeName == "" {
		return nil, fmt.Errorf("type name is required")
	}
	if !unicode.IsLower(rune(typeName[0])) {
		return nil, fmt.Errorf("first letter must be lowercase (private)")
	}

	return &Generator{
		Type:   typeName,
		Path:   path,
		values: make(map[string]int),
	}, nil
}

// SetLowerCase sets the lower case flag for marshal/unmarshal values
func (g *Generator) SetLowerCase(lower bool) {
	g.lowerCase = lower
}

// SetGenerateGetter sets the flag to generate getter methods for enum values
func (g *Generator) SetGenerateGetter(generate bool) {
	g.generateGetter = generate
}

// Parse reads the source directory and extracts enum information. it looks for const values
// that start with the enum type name, for example if type is "status", it will find all const values
// that start with "status". The values must use iota and be in sequence. The values map will contain
// the const name and its iota value, for example: {"statusActive": 1, "statusInactive": 2}
func (g *Generator) Parse(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		return fmt.Errorf("failed to parse directory: %w", err)
	}

	// process each package
	for _, pkg := range pkgs {
		g.pkgName = pkg.Name
		for _, file := range pkg.Files {
			g.parseFile(file)
		}
	}

	if len(g.values) == 0 {
		return fmt.Errorf("no const values found for type %s", g.Type)
	}

	return nil
}

// parseFile processes a single file for enum declarations
func (g *Generator) parseFile(file *ast.File) {

	parseConstBlock := func(decl *ast.GenDecl) {
		// extracts enum values from a const block
		var iotaVal int
		var lastExprWasIota bool
		var lastExplicitVal int

		for _, spec := range decl.Specs {
			vspec, ok := spec.(*ast.ValueSpec)
			if !ok || len(vspec.Names) == 0 {
				continue
			}

			// check if first name has our type prefix
			if !strings.HasPrefix(vspec.Names[0].Name, g.Type) {
				continue
			}

			// process all names in this spec
			for i, name := range vspec.Names {
				if name.Name == "_" { // skip placeholder values
					continue
				}

				// process value based on expression
				switch {
				case i < len(vspec.Values) && vspec.Values[i] != nil:
					// there's a value expression, try to extract the actual value
					switch expr := vspec.Values[i].(type) {
					case *ast.Ident:
						if expr.Name == "iota" {
							// the expression is an iota identifier
							g.values[name.Name] = iotaVal
							lastExprWasIota = true
						}
					case *ast.BasicLit:
						// try to extract literal value
						if val, err := convertLiteralToInt(expr); err == nil {
							g.values[name.Name] = val
							lastExplicitVal = val
							lastExprWasIota = false
						}
					}
				case lastExprWasIota:
					// if previous expr was iota and this one has no value, assume iota continues
					g.values[name.Name] = iotaVal
				default:
					// if this constant omits its expression following a non-iota value,
					// it repeats the previous expression (which means it gets the same value)
					g.values[name.Name] = lastExplicitVal
				}

				iotaVal++
			}
		}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.CONST {
			parseConstBlock(decl)
		}
		return true
	})
}

// convertLiteralToInt tries to convert a basic literal to an integer value
func convertLiteralToInt(lit *ast.BasicLit) (int, error) {
	switch lit.Kind {
	case token.INT:
		var val int
		if _, err := fmt.Sscanf(lit.Value, "%d", &val); err == nil {
			return val, nil
		}
		return 0, fmt.Errorf("cannot convert %s to int", lit.Value)
	default:
		return 0, fmt.Errorf("unsupported literal kind: %v", lit.Kind)
	}
}

// Generate creates the enum code file. it takes the const values found in Parse and creates
// a new type with json, sql and text marshaling support. the generated code includes:
//   - exported type with private name and value fields (e.g., Status{name: "active", value: 1})
//   - string representation (String method)
//   - text marshaling (MarshalText/UnmarshalText methods)
//   - sql marshaling (Value/Scan methods for driver.Valuer and sql.Scanner)
//   - parsing functions (Parse/Must variants)
//   - exported const values (e.g., StatusActive)
//   - helper functions to get all values and names
func (g *Generator) Generate() error {
	values := make([]Value, 0, len(g.values))
	names := make([]string, 0, len(g.values))
	// To avoid an undefined behavior for a Getter, we need to check if the values are unique
	if g.generateGetter {
		valuesCounter := make(map[int][]string)
		// check if multiple names exist for the same value
		for name, val := range g.values {
			if _, ok := valuesCounter[val]; !ok {
				valuesCounter[val] = []string{}
			}
			valuesCounter[val] = append(valuesCounter[val], name)
		}
		var errs []error
		for val, names := range valuesCounter {
			if len(names) > 1 {
				errs = append(
					errs, fmt.Errorf("multiple names for value %d: %s", val, strings.Join(names, ", ")),
				)
			}
		}
		if len(errs) > 0 {
			return errors.Join(errs...)
		}
	}
	// collect names for stable ordering
	for name := range g.values {
		names = append(names, name)
	}
	sort.Strings(names)

	// create values with proper name transformations for each case
	for _, name := range names {
		privateName := name
		// strip type prefix to get just the value name part (e.g., "Active" from "statusActive")
		nameWithoutPrefix := strings.TrimPrefix(privateName, g.Type)
		// create exported name by adding title-cased type (e.g., "StatusActive")
		publicName := titleCaser.String(g.Type) + nameWithoutPrefix
		values = append(values, Value{
			PrivateName: privateName,
			PublicName:  publicName,
			Name:        titleCaser.String(nameWithoutPrefix),
			Index:       g.values[name],
		})
	}

	// determine output package name: use directory name if path is set
	pkgName := g.pkgName
	if g.Path != "" {
		dir := filepath.Base(g.Path)
		// ensure package name is a valid go identifier
		if !isValidGoIdentifier(dir) {
			pkgName = "enum" // fallback to a safe name
		} else {
			pkgName = dir
		}
	}

	// prepare template data
	data := struct {
		Type           string
		Values         []Value
		Package        string
		LowerCase      bool
		GenerateGetter bool
	}{
		Type:           g.Type,
		Values:         values,
		Package:        pkgName,
		LowerCase:      g.lowerCase,
		GenerateGetter: g.generateGetter,
	}

	// execute template
	var buf bytes.Buffer
	if err := enumTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// format generated code
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format source: %w", err)
	}

	// ensure output directory exists
	if g.Path != "" {
		// get source directory permissions or use 0o755 as fallback
		dirPerm := os.FileMode(0o755)
		if info, err := os.Stat(filepath.Dir(g.Path)); err == nil && info.IsDir() {
			dirPerm = info.Mode().Perm()
		}

		if err := os.MkdirAll(g.Path, dirPerm); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// write generated code to file
	outputName := filepath.Join(g.Path, getFileNameForType(g.Type))

	// use source file permissions or 0o644 as fallback
	filePerm := os.FileMode(0o644)

	if err := os.WriteFile(outputName, src, filePerm); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// splitCamelCase splits a camel case string into words, it handles the sequential abbreviations
// and acronyms by treating them as single words.
// For example:
// "jobStatus" becomes ["job", "Status"].
// "internalIPAddress" becomes ["internal", "IP", "Address"].
// "internalIP" becomes ["internal", "IP"].
// "HTTPResponse" becomes ["HTTP", "Response"].
// "HTTP" is not split further.
func splitCamelCase(s string) []string {
	var words []string
	start := 0
	var prev rune
	for i, curr := range s {
		if i == 0 {
			prev = curr
			continue
		}
		_, width := utf8.DecodeRuneInString(s[i:])
		var next *rune
		if i+width < len(s) {
			nextr, _ := utf8.DecodeRuneInString(s[i+width:])
			next = &nextr
		}
		if (unicode.IsLower(prev) && unicode.IsUpper(curr)) ||
			(unicode.IsUpper(curr) && (next != nil && unicode.IsLower(*next))) {
			words = append(words, s[start:i])
			start = i
		}
		prev = curr
	}
	words = append(words, s[start:])
	return words
}

// getFileNameForType returns the file name for the generated enum code based on the type name.
// It converts the type name to snake case and appends "_enum.go" to it.
// For example, if the type name is "jobStatus", the file name will be "job_status_enum.go".
func getFileNameForType(typeName string) string {
	words := splitCamelCase(typeName)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}

	return strings.Join(words, "_") + "_enum.go"
}

// isValidGoIdentifier checks if a string is a valid Go identifier:
// - must start with a letter or underscore
// - can contain letters, digits, and underscores
func isValidGoIdentifier(s string) bool {
	if s == "" {
		return false
	}

	for i, c := range s {
		if i == 0 {
			if !unicode.IsLetter(c) && c != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
				return false
			}
		}
	}
	return true
}

var funcMap = template.FuncMap{
	"title":   titleCaser.String,
	"ToLower": strings.ToLower,
}

//go:embed enum.go.tmpl
var tmplt string

// template for the generated enum code, creates:
// - exported type with name and value fields
// - String method for fmt.Stringer
// - Marshal/Unmarshal for JSON support
// - Parse function with error handling
// - Must variant that panics on error
// - exported const values
// - Values and Names helper functions
var enumTemplate = template.Must(template.New("enum").Funcs(funcMap).Parse(tmplt))
