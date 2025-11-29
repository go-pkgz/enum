// Package generator provides a code generator for enum types. It reads Go source files and extracts enum values
// to generate a new type with text marshaling support by default. Optional flags add SQL, BSON (MongoDB), and YAML support.
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
	"strconv"
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
	Type           string                 // the private type name (e.g., "status")
	Path           string                 // output directory path
	values         map[string]*constValue // const values found with metadata
	pkgName        string                 // package name from source file
	lowerCase      bool                   // use lower case for marshal/unmarshal
	generateGetter bool                   // generate getter methods for enum values
	underlyingType string                 // underlying type (e.g., "uint8", "int", etc.)
	generateSQL    bool                   // generate SQL interfaces and imports
	generateBSON   bool                   // generate BSON interfaces and imports
	generateYAML   bool                   // generate YAML interfaces and imports
}

// constValue holds metadata about a const during parsing
type constValue struct {
	value   int       // the numeric value
	pos     token.Pos // source position for ordering
	aliases []string  // aliases from comment annotation
}

// constExprType represents the type of constant expression
type constExprType int

const (
	exprTypeNone   constExprType = iota // no expression type determined yet
	exprTypePlain                       // plain value without iota
	exprTypeIota                        // plain iota
	exprTypeIotaOp                      // iota with operation (e.g., iota + 1)
)

// iotaOperation encapsulates a binary operation with iota
type iotaOperation struct {
	op         token.Token // operation type (ADD, SUB, MUL, QUO)
	operand    int         // the non-iota operand
	iotaOnLeft bool        // whether iota is on the left side
}

// constParseState holds the state while parsing a const block
type constParseState struct {
	iotaVal      int            // current iota value for this const block
	lastExprType constExprType  // type of the last expression
	lastValue    int            // the last computed value
	iotaOp       *iotaOperation // current iota operation if any
}

// Value represents a single enum value
type Value struct {
	PrivateName string   // e.g., "statusActive"
	PublicName  string   // e.g., "StatusActive"
	Name        string   // e.g., "Active"
	Index       int      // enum index value
	Aliases     []string // e.g., ["rw", "read-write"] from // enum:alias=rw,read-write
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
		values: make(map[string]*constValue),
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

// SetGenerateSQL enables or disables generation of SQL interfaces
func (g *Generator) SetGenerateSQL(v bool) { g.generateSQL = v }

// SetGenerateBSON enables or disables generation of BSON interfaces
func (g *Generator) SetGenerateBSON(v bool) { g.generateBSON = v }

// SetGenerateYAML enables or disables generation of YAML interfaces
func (g *Generator) SetGenerateYAML(v bool) { g.generateYAML = v }

// Parse reads the source directory and extracts enum information. it looks for const values
// that start with the enum type name, for example if type is "status", it will find all const values
// that start with "status". The values must use iota and be in sequence. The values map will contain
// the const name and its iota value, for example: {"statusActive": 1, "statusInactive": 2}
func (g *Generator) Parse(dir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
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
	// first pass: look for the type declaration to get underlying type
	g.extractUnderlyingType(file)

	// second pass: extract const values
	ast.Inspect(file, func(n ast.Node) bool {
		if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.CONST {
			g.parseConstBlock(decl)
		}
		return true
	})
}

// extractUnderlyingType finds the type declaration and extracts its underlying type
func (g *Generator) extractUnderlyingType(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.TYPE {
			for _, spec := range decl.Specs {
				if tspec, ok := spec.(*ast.TypeSpec); ok && tspec.Name.Name == g.Type {
					// found our type, extract the underlying type
					if ident, ok := tspec.Type.(*ast.Ident); ok {
						g.underlyingType = ident.Name
					}
				}
			}
		}
		return true
	})
}

// parseConstBlock extracts enum values from a const block
func (g *Generator) parseConstBlock(decl *ast.GenDecl) {
	state := &constParseState{}

	for _, spec := range decl.Specs {
		vspec, ok := spec.(*ast.ValueSpec)
		if !ok || len(vspec.Names) == 0 {
			continue
		}

		// parse aliases from inline comment (vspec.Comment is the inline comment)
		aliases := parseAliasComment(vspec.Comment)

		// process all names in this spec
		for i, name := range vspec.Names {
			// skip underscore placeholders
			if name.Name == "_" {
				continue
			}

			// only process names with our type prefix
			if !strings.HasPrefix(name.Name, g.Type) {
				continue
			}

			// process value based on expression
			enumValue := g.processConstValue(vspec, i, state)

			// store the value with its position and aliases
			g.values[name.Name] = &constValue{
				value:   enumValue,
				pos:     name.Pos(),
				aliases: aliases,
			}
		}

		// always increment iota after each value spec
		state.iotaVal++
	}
}

// processConstValue extracts the value for a single constant
func (g *Generator) processConstValue(vspec *ast.ValueSpec, index int, state *constParseState) int {
	// handle explicit expression if present
	if index < len(vspec.Values) && vspec.Values[index] != nil {
		return g.processExplicitValue(vspec.Values[index], state)
	}

	// handle implicit expression based on previous state
	return g.processImplicitValue(state)
}

// processExplicitValue handles a constant with an explicit value expression
func (g *Generator) processExplicitValue(expr ast.Expr, state *constParseState) int {
	switch e := expr.(type) {
	case *ast.Ident:
		if e.Name == "iota" {
			state.lastExprType = exprTypeIota
			state.lastValue = state.iotaVal
			state.iotaOp = nil
			return state.iotaVal
		}
	case *ast.BasicLit:
		if val, err := ConvertLiteralToInt(e); err == nil {
			state.lastExprType = exprTypePlain
			state.lastValue = val
			state.iotaOp = nil
			return val
		}
	case *ast.BinaryExpr:
		if val, op := g.processBinaryExpr(e, state); op != nil {
			state.lastExprType = exprTypeIotaOp
			state.lastValue = val
			state.iotaOp = op
			return val
		} else if val != 0 || op == nil {
			// plain binary expression without iota
			state.lastExprType = exprTypePlain
			state.lastValue = val
			state.iotaOp = nil
			return val
		}
	case *ast.UnaryExpr:
		// handle negative numbers like -1
		if e.Op == token.SUB {
			if lit, ok := e.X.(*ast.BasicLit); ok {
				if val, err := ConvertLiteralToInt(lit); err == nil {
					state.lastExprType = exprTypePlain
					state.lastValue = -val
					state.iotaOp = nil
					return -val
				}
				// if conversion fails, fall through to return 0 (same as BasicLit case)
			}
		}
	}
	return 0
}

// processImplicitValue handles a constant without an explicit value
func (g *Generator) processImplicitValue(state *constParseState) int {
	switch state.lastExprType {
	case exprTypeIota:
		// plain iota continues
		return state.iotaVal
	case exprTypeIotaOp:
		// apply the operation with current iota
		return g.applyIotaOperation(state.iotaOp, state.iotaVal)
	default:
		// repeat last plain value
		return state.lastValue
	}
}

// processBinaryExpr processes a binary expression and returns the value and operation if it uses iota
func (g *Generator) processBinaryExpr(expr *ast.BinaryExpr, state *constParseState) (int, *iotaOperation) {
	val, usesIota, err := EvaluateBinaryExpr(expr, state.iotaVal)
	if err != nil {
		return 0, nil
	}

	if !usesIota {
		return val, nil
	}

	// extract operation details for iota expressions
	op := &iotaOperation{op: expr.Op}

	if ident, ok := expr.X.(*ast.Ident); ok && ident.Name == "iota" {
		// iota op value
		op.iotaOnLeft = true
		if lit, ok := expr.Y.(*ast.BasicLit); ok {
			if opVal, err := ConvertLiteralToInt(lit); err == nil {
				op.operand = opVal
			}
		}
	} else if ident, ok := expr.Y.(*ast.Ident); ok && ident.Name == "iota" {
		// value op iota
		op.iotaOnLeft = false
		if lit, ok := expr.X.(*ast.BasicLit); ok {
			if opVal, err := ConvertLiteralToInt(lit); err == nil {
				op.operand = opVal
			}
		}
	}

	return val, op
}

// applyIotaOperation applies a stored operation to a new iota value
func (g *Generator) applyIotaOperation(op *iotaOperation, iotaVal int) int {
	if op == nil {
		return iotaVal
	}

	switch op.op {
	case token.ADD:
		return iotaVal + op.operand
	case token.SUB:
		if op.iotaOnLeft {
			return iotaVal - op.operand
		}
		return op.operand - iotaVal
	case token.MUL:
		return iotaVal * op.operand
	case token.QUO:
		if op.operand != 0 {
			if op.iotaOnLeft {
				return iotaVal / op.operand
			}
			// note: integer division by iota could be 0 for large iota values
			if iotaVal != 0 {
				return op.operand / iotaVal
			}
		}
		return 0 // division by zero
	}
	return iotaVal
}

// ConvertLiteralToInt tries to convert a basic literal to an integer value
func ConvertLiteralToInt(lit *ast.BasicLit) (int, error) {
	switch lit.Kind {
	case token.INT:
		var val int
		if _, err := fmt.Sscanf(lit.Value, "%d", &val); err == nil {
			return val, nil
		}
		return 0, fmt.Errorf("cannot convert %s to int", lit.Value)
	case token.CHAR:
		// handle character literals like 'A'
		// strconv.Unquote handles all escape sequences properly
		unquoted, err := strconv.Unquote(lit.Value)
		if err != nil {
			return 0, fmt.Errorf("cannot parse character literal %s: %w", lit.Value, err)
		}
		// use utf8.DecodeRuneInString for safer UTF-8 handling
		r, size := utf8.DecodeRuneInString(unquoted)
		if r == utf8.RuneError {
			return 0, fmt.Errorf("invalid UTF-8 in character literal %s", lit.Value)
		}
		if size != len(unquoted) {
			return 0, fmt.Errorf("character literal %s contains multiple characters", lit.Value)
		}
		return int(r), nil
	default:
		return 0, fmt.Errorf("unsupported literal kind: %v", lit.Kind)
	}
}

// EvaluateBinaryExpr evaluates binary expressions like iota + 1
// Returns:
// - value: the computed value of the expression
// - usesIota: whether the expression uses iota
// - error: any error encountered
func EvaluateBinaryExpr(expr *ast.BinaryExpr, iotaVal int) (value int, usesIota bool, err error) {
	// handle left side of expression
	var leftVal int
	var leftIsIota bool

	switch left := expr.X.(type) {
	case *ast.Ident:
		if left.Name == "iota" {
			leftVal = iotaVal
			leftIsIota = true
		} else {
			return 0, false, fmt.Errorf("unsupported identifier in binary expression: %s", left.Name)
		}
	case *ast.BasicLit:
		var err error
		leftVal, err = ConvertLiteralToInt(left)
		if err != nil {
			return 0, false, err
		}
	default:
		return 0, false, fmt.Errorf("unsupported expression type on left side: %T", left)
	}

	// handle right side of expression
	var rightVal int
	var rightIsIota bool

	switch right := expr.Y.(type) {
	case *ast.Ident:
		if right.Name == "iota" {
			rightVal = iotaVal
			rightIsIota = true
		} else {
			return 0, false, fmt.Errorf("unsupported identifier in binary expression: %s", right.Name)
		}
	case *ast.BasicLit:
		var err error
		rightVal, err = ConvertLiteralToInt(right)
		if err != nil {
			return 0, false, err
		}
	default:
		return 0, false, fmt.Errorf("unsupported expression type on right side: %T", right)
	}

	// check if expression uses iota
	usesIota = leftIsIota || rightIsIota

	// evaluate the expression based on the operator
	switch expr.Op {
	case token.ADD:
		value = leftVal + rightVal
	case token.SUB:
		value = leftVal - rightVal
	case token.MUL:
		value = leftVal * rightVal
	case token.QUO:
		if rightVal == 0 {
			return 0, false, fmt.Errorf("division by zero")
		}
		value = leftVal / rightVal
	default:
		return 0, false, fmt.Errorf("unsupported binary operator: %v", expr.Op)
	}

	return value, usesIota, nil
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
	// validate aliases: no duplicates and no conflicts with canonical names
	if err := g.validateAliases(); err != nil {
		return err
	}

	// to avoid an undefined behavior for a Getter, we need to check if the values are unique
	if g.generateGetter {
		valuesCounter := make(map[int][]string)
		// check if multiple names exist for the same value
		for name, cv := range g.values {
			if _, ok := valuesCounter[cv.value]; !ok {
				valuesCounter[cv.value] = []string{}
			}
			valuesCounter[cv.value] = append(valuesCounter[cv.value], name)
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

	// collect entries for sorting by position
	type entry struct {
		name string
		cv   *constValue
	}
	entries := make([]entry, 0, len(g.values))
	for name, cv := range g.values {
		entries = append(entries, entry{name: name, cv: cv})
	}

	// sort by source position to preserve declaration order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].cv.pos < entries[j].cv.pos
	})

	// create values with proper name transformations for each case
	values := make([]Value, 0, len(entries))
	for _, e := range entries {
		privateName := e.name
		// strip type prefix to get just the value name part (e.g., "Active" from "statusActive")
		nameWithoutPrefix := strings.TrimPrefix(privateName, g.Type)
		// create exported name by adding title-cased type (e.g., "StatusActive")
		publicName := titleCaser.String(g.Type) + nameWithoutPrefix
		values = append(values, Value{
			PrivateName: privateName,
			PublicName:  publicName,
			Name:        titleCaser.String(nameWithoutPrefix),
			Index:       e.cv.value,
			Aliases:     e.cv.aliases,
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
		UnderlyingType string
		GenerateSQL    bool
		GenerateBSON   bool
		GenerateYAML   bool
	}{
		Type:           g.Type,
		Values:         values,
		Package:        pkgName,
		LowerCase:      g.lowerCase,
		GenerateGetter: g.generateGetter,
		UnderlyingType: g.underlyingType,
		GenerateSQL:    g.generateSQL,
		GenerateBSON:   g.generateBSON,
		GenerateYAML:   g.generateYAML,
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

// validateAliases checks for duplicate aliases and conflicts with canonical names
func (g *Generator) validateAliases() error {
	// collect all canonical names first (case-insensitive)
	canonicalNames := make(map[string]string) // lowercase -> constant name
	for name := range g.values {
		nameWithoutPrefix := strings.TrimPrefix(name, g.Type)
		canonicalNames[strings.ToLower(nameWithoutPrefix)] = name
	}

	// validate aliases
	aliasToConst := make(map[string]string) // lowercase alias -> constant name
	var errs []error

	for name, cv := range g.values {
		for _, alias := range cv.aliases {
			lowerAlias := strings.ToLower(alias)

			// check if alias conflicts with a DIFFERENT constant's canonical name
			if existingName, ok := canonicalNames[lowerAlias]; ok && existingName != name {
				errs = append(errs, fmt.Errorf("alias %q for %s conflicts with canonical name of %s", alias, name, existingName))
				continue
			}

			// check for duplicate aliases
			if existingName, ok := aliasToConst[lowerAlias]; ok {
				errs = append(errs, fmt.Errorf("duplicate alias %q: used by both %s and %s", alias, existingName, name))
				continue
			}

			aliasToConst[lowerAlias] = name
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// parseAliasComment extracts aliases from an inline comment like "// enum:alias=rw,read-write"
func parseAliasComment(comment *ast.CommentGroup) []string {
	if comment == nil {
		return nil
	}
	for _, c := range comment.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if strings.HasPrefix(text, "enum:alias=") {
			aliasStr := strings.TrimPrefix(text, "enum:alias=")
			if aliasStr == "" {
				return nil
			}
			aliases := strings.Split(aliasStr, ",")
			result := make([]string, 0, len(aliases))
			for _, a := range aliases {
				if trimmed := strings.TrimSpace(a); trimmed != "" {
					result = append(result, trimmed)
				}
			}
			if len(result) == 0 {
				return nil
			}
			return result
		}
	}
	return nil
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
