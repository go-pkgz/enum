// Package generator provides a code generator for enum types. It reads Go source files and extracts enum values
// to generate a new type with json, bson and text marshaling support.
package generator

import (
	"bytes"
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

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var titleCaser = cases.Title(language.English, cases.NoLower)

// Generator holds the data needed for enum code generation
type Generator struct {
	Type      string         // the private type name (e.g., "status")
	Path      string         // output directory path
	values    map[string]int // const values found
	pkgName   string         // package name from source file
	lowerCase bool           // use lower case for marshal/unmarshal
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
		for _, spec := range decl.Specs {
			vspec, ok := spec.(*ast.ValueSpec)
			if !ok || len(vspec.Names) == 0 {
				continue
			}

			// check if first name has our type prefix
			if !strings.HasPrefix(vspec.Names[0].Name, g.Type) {
				continue
			}

			// process all names in this const group
			for _, name := range vspec.Names {
				if name.Name != "_" { // skip placeholder values
					g.values[name.Name] = iotaVal
					iotaVal++
				}
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
		Type      string
		Values    []Value
		Package   string
		LowerCase bool
	}{
		Type:      g.Type,
		Values:    values,
		Package:   pkgName,
		LowerCase: g.lowerCase,
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
		if err := os.MkdirAll(g.Path, 0o700); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// write generated code to file
	outputName := filepath.Join(g.Path, getFileNameForType(g.Type))
	if err := os.WriteFile(outputName, src, 0o600); err != nil {
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
		var next *rune
		if i+1 < len(s) {
			nextr := rune(s[i+1])
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
// For example, if the type name is "JobStatus", the file name will be "job_status_enum.go".
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

// template for the generated enum code, creates:
// - exported type with name and value fields
// - String method for fmt.Stringer
// - Marshal/Unmarshal for JSON support
// - Parse function with error handling
// - Must variant that panics on error
// - exported const values
// - Values and Names helper functions
var enumTemplate = template.Must(template.New("enum").Funcs(funcMap).Parse(`// Code generated by enum generator; DO NOT EDIT.
package {{.Package}}

import (
	"fmt"

	"database/sql/driver"
	{{- if .LowerCase | not }}
	"strings"
	{{- end}}
)

// {{.Type | title}} is the exported type for the enum
type {{.Type | title}} struct {
	name  string
	value int
}

func (e {{.Type | title}}) String() string { return e.name }

// MarshalText implements encoding.TextMarshaler
func (e {{.Type | title}}) MarshalText() ([]byte, error) {
	return []byte(e.name), nil
}

// UnmarshalText implements encoding.TextUnmarshaler
func (e *{{.Type | title}}) UnmarshalText(text []byte) error {
	var err error
	*e, err = Parse{{.Type | title}}(string(text))
	return err
}

// Value implements the driver.Valuer interface
func (e {{.Type | title}}) Value() (driver.Value, error) {
	return e.name, nil
}

// Scan implements the sql.Scanner interface
func (e *{{.Type | title}}) Scan(value interface{}) error {
	if value == nil {
		*e = {{.Type | title}}Values()[0]
		return nil
	}

	str, ok := value.(string)
	if !ok {
		if b, ok := value.([]byte); ok {
			str = string(b)
		} else {
			return fmt.Errorf("invalid {{.Type}} value: %v", value)
		}
	}

	val, err := Parse{{.Type | title}}(str)
	if err != nil {
		return err
	}

	*e = val
	return nil
}

// Parse{{.Type | title}} converts string to {{.Type}} enum value
func Parse{{.Type | title}}(v string) ({{.Type | title}}, error) {
{{if .LowerCase}}
	switch v {
	{{range .Values -}}
	case "{{.Name | ToLower}}":
		return {{.PublicName}}, nil
	{{end}}
	}
{{else}}
	switch strings.ToLower(v) {
	{{range .Values -}}
	case strings.ToLower("{{.Name}}"):
		return {{.PublicName}}, nil
	{{end}}
	}
{{end}}
	return {{.Type | title}}{}, fmt.Errorf("invalid {{.Type}}: %s", v)
}

// Must{{.Type | title}} is like Parse{{.Type | title}} but panics if string is invalid
func Must{{.Type | title}}(v string) {{.Type | title}} {
	r, err := Parse{{.Type | title}}(v)
	if err != nil {
		panic(err)
	}
	return r
}

// Public constants for {{.Type}} values
var (
{{range .Values -}}
	{{.PublicName}} = {{$.Type | title}}{name: "{{if $.LowerCase}}{{.Name | ToLower}}{{else}}{{.Name}}{{end}}", value: {{.Index}}}
{{end -}}
)

// {{.Type | title}}Values returns all possible enum values
func {{.Type | title}}Values() []{{.Type | title}} {
	return []{{.Type | title}}{
	{{range .Values -}}
		{{.PublicName}},
	{{end -}}
	}
}

// {{.Type | title}}Names returns all possible enum names
func {{.Type | title}}Names() []string {
	return []string{
	{{range .Values -}}
		"{{if $.LowerCase}}{{.Name | ToLower}}{{else}}{{.Name}}{{end}}",
	{{end -}}
	}
}`))
