package generator

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// constVal returns the parsed value of a constant as int64
func constVal(t *testing.T, gen *Generator, name string) int64 {
	t.Helper()
	cv, ok := gen.values[name]
	require.True(t, ok, "constant %s not found", name)
	v, exact := constant.Int64Val(cv.value)
	require.True(t, exact, "value of %s does not fit int64: %s", name, cv.value.ExactString())
	return v
}

// resolveSrc parses a go source and resolves a single constant from it to its integer value
func resolveSrc(t *testing.T, src, name string) (constant.Value, error) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "src.go", src, parser.ParseComments)
	require.NoError(t, err)
	r := newConstResolver()
	r.addFile(file)
	v, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	return toInt(v.value)
}

func TestConstResolverValues(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{"decimal", "42", "42"},
		{"negative", "-42", "-42"},
		{"unary plus", "+42", "42"},
		{"hex", "0x10", "16"},
		{"hex upper", "0XFF", "255"},
		{"binary", "0b1010", "10"},
		{"octal", "0o17", "15"},
		{"octal legacy", "017", "15"},
		{"underscored", "1_000_000", "1000000"},
		{"underscored hex", "0xff_ff", "65535"},
		{"char", "'A'", "65"},
		{"char escape", "'\\n'", "10"},
		{"char byte escape", "'\\x80'", "128"},
		{"char unicode", "'\\u00e9'", "233"},
		{"shift left", "1 << 4", "16"},
		{"shift right", "256 >> 4", "16"},
		{"shift by constant", "1 << 62", "4611686018427387904"},
		{"bitwise or", "5 | 2", "7"},
		{"bitwise and", "6 & 3", "2"},
		{"bitwise xor", "7 ^ 2", "5"},
		{"bitwise clear", "7 &^ 2", "5"},
		{"bitwise not", "^0", "-1"},
		{"remainder", "10 % 3", "1"},
		{"integer division", "7 / 2", "3"},
		{"nested", "(1 + 2) * (3 + 4)", "21"},
		{"deeply nested", "((1 << 3) | (1 << 1)) - 2", "8"},
		{"conversion builtin", "uint8(3)", "3"},
		{"conversion nested", "uint16(1 << 9)", "512"},
		{"complement of unsigned", "^uint8(0)", "255"},
		{"complement of uint64", "^uint64(0)", "18446744073709551615"},
		{"complement of signed", "^int8(0)", "-1"},
		{"complement of int", "^int(0)", "-1"},
		{"complement of untyped", "^0", "-1"},
		{"float with integer value", "1.5 * 2", "3"},
		{"float division", "5.0 / 2 * 2", "5"},
		{"len of a string", `len("abc")`, "3"},
		{"len with parentheses", `(len)(("abc"))`, "3"},
		{"conversion with parentheses", "(uint8)(7)", "7"},
		{"character arithmetic", "'a' + 1", "98"},
		{"right shift of a negative", "-4 >> 1", "-2"},
		{"beyond int64", "1<<62*4 - 1", "18446744073709551615"},
		{"shift right past the width", "1 >> 1000", "0"},
		{"shift right of a negative past the width", "-1 >> 1000", "-1"},
		{"float conversion", "1 / float64(2) * 6", "3"},
		{"len of a concatenation", `len("a" + "b")`, "2"},
		{"typed integer and float operand", "uint8(5) & 3.0", "1"},
		{"min", "min(0, 1)", "0"},
		{"max", "max(3, 2, 7)", "7"},
		{"min of mixed kinds", "min(1, 2.5)", "1"},
		{"float32 rounds", "int(float32(16777217))", "16777216"},
		{"float64 rounds", "int(float64(1<<62 + 1))", "4611686018427387904"},
		{"typed float arithmetic", "int(float32(1) / 3 * 3)", "1"},
		{"string of a code point", `len(string(0x100))`, "2"},
		{"min of a typed argument", "^max(2, uint8(1))", "253"},
		{"max of a typed argument", "min(uint8(7), 9) + 1", "8"},
		{"max with an untyped float", "max(5, 4.0) / 2 * 10", "25"},
		{"min with an untyped float", "min(5, 6.0) / 2 * 10", "25"},
		{"max of integers only", "max(5, 4) / 2 * 10", "20"},
		{"string of a value out of range", "len(string(-1))", "3"},
		{"shift right of a wide value", "1<<70 >> 2000", "0"},
		// the compiler rejects a shift count this large outright, the result is still 0
		{"shift right past the bound", "1 >> 2000000", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := resolveSrc(t, "package p\nconst x = "+tt.expr+"\n", "x")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, v.ExactString())
		})
	}
}

func TestConstResolverErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		errText string
	}{
		{"division by zero", "const x = 1 / 0", "division by zero"},
		{"remainder by zero", "const x = 1 % 0", "division by zero"},
		{"negative shift", "const x = 1 << -1", "negative shift count"},
		{"huge shift", "const x = 1 << 100000", "too large"},
		{"string literal", `const x = "str"`, "not an integer"},
		{"len of a number", "const x = len(1)", "only supported for a string"},
		{"float literal", "const x = 3.14", "not an integer"},
		{"fractional expression", "const x = 7.0 / 2", "not an integer"},
		{"fractional sum", "const x = 1 + 1.5", "not an integer"},
		{"unknown constant", "const x = missing + 1", "unknown constant missing"},
		{"package reference", "const x = math.MaxInt8", "unsupported expression"},
		{"unsupported function", "const x = real(3+4i)", "unsupported call to real"},
		{"len of two arguments", `const x = len("a", "b")`, "single argument"},
		{"conversion out of range", "const x = uint8(300)", "overflows uint8"},
		{"conversion of a negative", "const x = uint8(-1)", "negative"},
		{"typed declaration out of range", "const x int8 = 200", "overflows int8"},
		{"complement of uint", "const x = ^uint(0)", "depends on the target architecture"},
		{"complement of uintptr", "const x = ^uintptr(0)", "depends on the target architecture"},
		{"comparison operator", "const x = 1 < 2", "unsupported binary operator"},
		{"self reference", "const x = x + 1", "refers to itself"},
		{"reference cycle", "const (\n\tx = y\n\ty = x\n)", "refers to itself"},
		{"failing operand of a negation", "const x = -missing", "unknown constant missing"},
		{"complement of a string", `const x = ^"str"`, "not an integer"},
		{"failing right operand", "const x = 1 + missing", "unknown constant missing"},
		{"operand out of range for the type", "const x = uint8(1) + 300", "overflows uint8"},
		{"string added to a number", `const x = 1 + "s"`, "not a number"},
		{"number added to a string", `const x = "s" + 1`, "not a number"},
		{"fractional remainder", "const x = 1.5 % 2", "not an integer"},
		{"remainder by a fraction", "const x = 2 % 1.5", "not an integer"},
		{"shift of a string", `const x = "s" << 1`, "not an integer"},
		{"failing shift count", "const x = 1 << missing", "unknown constant missing"},
		{"fractional shift count", "const x = 1 << 1.5", "not an integer"},
		{"conversion with two arguments", "const x = uint8(1, 2)", "unsupported call expression"},
		{"min without arguments", "const x = min()", "at least one argument"},
		{"failing min argument", "const x = min(missing, 1)", "unknown constant missing"},
		{"min of a string", `const x = min("a", 1)`, "not a number"},
		{"failing len argument", "const x = len(missing)", "unknown constant missing"},
		{"failing conversion argument", "const x = uint8(missing)", "unknown constant missing"},
		{"fractional conversion", "const x = uint8(1.5)", "not an integer"},
		{"float conversion of a string", `const x = float64("s")`, "not a number"},
		{"string conversion of a fraction", "const (\n\tx = str(1.5)\n)\ntype str string", "not a string"},
		{"call of an expression", "const x = (1 + 1)(2)", "unsupported call expression"},
		{"failing left operand of a typed sum", "const x = 300 + uint8(1)", "overflows uint8"},
		{"alias cycle", "const x = ^a(0)\ntype a = b\ntype b = a", "unsupported call to a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveSrc(t, "package p\n"+tt.src+"\n", "x")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errText)
		})
	}
}

func TestConstResolverReferences(t *testing.T) {
	src := `package p

const base = 100

const (
	first  = base + iota // 100
	second               // 101
)

const shifted = first << 1

const converted = myType(second)

type myType int
`
	for name, expected := range map[string]string{
		"base": "100", "first": "100", "second": "101", "shifted": "200", "converted": "101",
	} {
		v, err := resolveSrc(t, src, name)
		require.NoError(t, err, name)
		assert.Equal(t, expected, v.ExactString(), name)
	}
}

func TestConstResolverSpecWithoutValue(t *testing.T) {
	// a const spec with no expression repeats the previous one, iota moves on
	src := `package p
const (
	a = 1 << iota
	b
	c
	_
	e
)
`
	for name, expected := range map[string]string{"a": "1", "b": "2", "c": "4", "e": "16"} {
		v, err := resolveSrc(t, src, name)
		require.NoError(t, err, name)
		assert.Equal(t, expected, v.ExactString(), name)
	}
}

func TestConstResolverTypedConstants(t *testing.T) {
	src := `package p

type flag uint8

const (
	flagNone flag = 0
	flagOne  flag = 1
)

const (
	all     = ^flagNone     // 255, the complement is taken within uint8
	notOne  = ^flagOne      // 254
	untyped = ^0            // -1, no type to complement within
	wide    = ^myUint64(0)  // 18446744073709551615
)

type myUint64 = uint64
`
	for name, expected := range map[string]string{"all": "255", "notOne": "254", "untyped": "-1", "wide": "18446744073709551615"} {
		v, err := resolveSrc(t, src, name)
		require.NoError(t, err, name)
		assert.Equal(t, expected, v.ExactString(), name)
	}
}

func TestConstResolverNumberTypes(t *testing.T) {
	// arithmetic follows the type of its operands, division by a float is not integer division
	src := `package p

type uint8 = uint16
type lvl int32
type l1 = l2
type l2 = l3
type l3 = l4
type l4 = l5
type l5 = l6
type l6 = l7
type l7 = l8
type l8 = l9
type l9 = l10
type l10 = l11
type l11 = uint8

const (
	d      float64 = 2
	viaD           = 5 / d * 2   // 5, not 4
	whole  int     = 5
	halved         = whole / 2.0 // 2, a typed integer divides as an integer
	tl     lvl     = 7
	scaled         = tl*2 + 1    // 15
	shadow         = ^uint8(0)   // 65535, the package declaration shadows the builtin
	text           = "hello"
	size           = len(text)
	named          = len(str("abcd"))
	paren  (uint8) = 0
	compl          = ^paren      // 65535, uint8 here is the package declaration
	chained        = ^l1(0)      // the same, reached through eleven aliases
)

type str (string)
`
	for name, expected := range map[string]string{
		"viaD": "5", "halved": "2", "scaled": "15", "shadow": "65535", "size": "5", "named": "4",
		"compl": "65535", "chained": "65535",
	} {
		v, err := resolveSrc(t, src, name)
		require.NoError(t, err, name)
		assert.Equal(t, expected, v.ExactString(), name)
	}
}

func TestConstResolverIgnoresLocalConstants(t *testing.T) {
	// a constant declared inside a function is out of scope for the enum values
	src := `package p

const base = 1

func f() {
	const base = 2
	_ = base
}

const derived = base + 10
`
	v, err := resolveSrc(t, src, "derived")
	require.NoError(t, err)
	assert.Equal(t, "11", v.ExactString())

	_, err = resolveSrc(t, src, "missing")
	require.Error(t, err)
}

func TestCheckIntRange(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		underlyingType string
		wantErr        string
	}{
		{"int8 max", "127", "int8", ""},
		{"int8 min", "-128", "int8", ""},
		{"int8 overflow", "128", "int8", "overflows int8"},
		{"int8 underflow", "-129", "int8", "overflows int8"},
		{"uint8 max", "255", "uint8", ""},
		{"uint8 overflow", "256", "uint8", "overflows uint8"},
		{"uint8 negative", "-1", "uint8", "negative"},
		{"uint64 max", "18446744073709551615", "uint64", ""},
		{"uint64 overflow", "18446744073709551616", "uint64", "overflows uint64"},
		{"int64 max", "9223372036854775807", "int64", ""},
		{"int64 overflow", "9223372036854775808", "int64", "overflows int64"},
		{"default type is int", "9223372036854775808", "", "overflows int"},
		{"unknown type is skipped", "9223372036854775808", "myType", ""},
		{"rune", "1114111", "rune", ""},
		{"byte overflow", "256", "byte", "overflows byte"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := constant.MakeFromLiteral(tt.value, token.INT, 0)
			require.Equal(t, constant.Int, v.Kind())
			err := checkIntRange(v, tt.underlyingType)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestConstResolverDeclarations(t *testing.T) {
	// a name declared twice keeps the first declaration, which is what a compiling package has
	v, err := resolveSrc(t, "package p\nconst x = 1\nconst x = 2\n", "x")
	require.NoError(t, err)
	assert.Equal(t, "1", v.ExactString())

	// specs that are not value or type declarations are skipped
	r := newConstResolver()
	r.addFile(&ast.File{
		Name: &ast.Ident{Name: "p"},
		Decls: []ast.Decl{
			&ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.ImportSpec{}}},
			&ast.GenDecl{Tok: token.CONST, Specs: []ast.Spec{&ast.ImportSpec{}}},
			&ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{&ast.ImportSpec{}}},
			&ast.FuncDecl{Name: &ast.Ident{Name: "f"}},
		},
	})
	assert.Empty(t, r.decls)
	assert.Empty(t, r.types)
}

func TestConstResolverUnsupportedNodes(t *testing.T) {
	r := newConstResolver()

	_, err := r.eval(&ast.FuncLit{}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported expression")

	_, err = r.eval(&ast.UnaryExpr{Op: token.NOT, X: &ast.BasicLit{Kind: token.INT, Value: "1"}}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported unary operator")

	_, err = r.eval(&ast.CallExpr{Fun: &ast.SelectorExpr{}, Args: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}}}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported call expression")

	_, err = literalValue(&ast.BasicLit{Kind: token.CHAR, Value: "'ab'"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid literal")

	_, err = literalValue(&ast.BasicLit{Kind: token.IMAG, Value: "1i"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a number")

	for _, lit := range []*ast.BasicLit{
		{Kind: token.INT, Value: "12abc"},
		{Kind: token.FLOAT, Value: "1.2.3"},
		{Kind: token.CHAR, Value: "'"},
		{Kind: token.CHAR, Value: "abc"},
		{Kind: token.CHAR, Value: `'\q'`},
	} {
		_, err = literalValue(lit)
		require.Error(t, err, lit.Value)
		assert.Contains(t, err.Error(), "invalid literal", lit.Value)
	}

	_, err = r.resolve("nothing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown constant nothing")
}

// parseSrc writes a single source file to a temp dir and parses it with the generator
func parseSrc(t *testing.T, typeName, src string) (*Generator, error) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "enum.go"), []byte(src), 0o600))
	gen, err := New(typeName, dir)
	require.NoError(t, err)
	err = gen.Parse(dir)
	return gen, err
}

func TestParseBitmaskEnum(t *testing.T) {
	src := `package test
type perm uint8
const (
	permRead perm = 1 << iota
	permWrite
	permExecute
)
`
	gen, err := parseSrc(t, "perm", src)
	require.NoError(t, err)

	assert.Equal(t, int64(1), constVal(t, gen, "permRead"))
	assert.Equal(t, int64(2), constVal(t, gen, "permWrite"))
	assert.Equal(t, int64(4), constVal(t, gen, "permExecute"))

	gen.SetGenerateGetter(true)
	require.NoError(t, gen.Generate())
	content, err := os.ReadFile(filepath.Join(gen.Path, "perm_enum.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), `Perm{name: "Write", value: 2}`)
	assert.Contains(t, string(content), `Perm{name: "Execute", value: 4}`)
	assert.Contains(t, string(content), "case 4:")
}

func TestParseNonDecimalLiterals(t *testing.T) {
	src := `package test
type code uint16
const (
	codeA code = 0x10
	codeB code = 0b1000_0000
	codeC code = 1_000
)
`
	gen, err := parseSrc(t, "code", src)
	require.NoError(t, err)

	assert.Equal(t, int64(16), constVal(t, gen, "codeA"))
	assert.Equal(t, int64(128), constVal(t, gen, "codeB"))
	assert.Equal(t, int64(1000), constVal(t, gen, "codeC"))
}

func TestParseWithConstantReference(t *testing.T) {
	src := `package test

const offset = 1 << 8

type code uint16
const (
	codeA code = offset
	codeB code = offset + 1
)
`
	gen, err := parseSrc(t, "code", src)
	require.NoError(t, err)

	assert.Equal(t, int64(256), constVal(t, gen, "codeA"))
	assert.Equal(t, int64(257), constVal(t, gen, "codeB"))
}

func TestParseValuesBeyondInt64(t *testing.T) {
	src := `package test
type flag uint64
const (
	flagNone flag = 0
	flagHigh flag = 1 << 63
	flagAll  flag = 1<<64 - 1
)
`
	gen, err := parseSrc(t, "flag", src)
	require.NoError(t, err)

	assert.Equal(t, "9223372036854775808", gen.values["flagHigh"].value.ExactString())
	assert.Equal(t, "18446744073709551615", gen.values["flagAll"].value.ExactString())

	require.NoError(t, gen.Generate())
	content, err := os.ReadFile(filepath.Join(gen.Path, "flag_enum.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "value: 9223372036854775808")
	assert.Contains(t, string(content), "value: 18446744073709551615")
}

func TestParseDeclaredTypeApplied(t *testing.T) {
	// the declared type of a constant shapes its value, a float one holds it at the width of the type
	src := `package test
type status int32
const (
	statusRounded (float32) = 16777217
	statusPlain   status    = 5
)
`
	gen, err := parseSrc(t, "status", src)
	require.NoError(t, err)
	assert.Equal(t, int64(16777216), constVal(t, gen, "statusRounded"))
	assert.Equal(t, int64(5), constVal(t, gen, "statusPlain"))
}

func TestParseParenthesisedUnderlyingType(t *testing.T) {
	// the underlying type is read through parentheses, a negative value does not fit it
	src := `package test
type code (uint8)
const (
	codeA code = 200
	codeB code = -1
)
`
	_, err := parseSrc(t, "code", src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "const codeB: value -1 is negative but the type is uint8")
}

func TestParseUntypedValueOutOfRange(t *testing.T) {
	// a constant without a type of its own still has to fit the underlying type of the enum
	src := `package test
type small int8
const (
	smallA small = 100
	smallB       = 200
)
`
	_, err := parseSrc(t, "small", src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "const smallB: value 200 overflows int8")
}

func TestParseConstWithoutValue(t *testing.T) {
	src := `package test
type status int
const (
	statusA
)
`
	_, err := parseSrc(t, "status", src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no value for const statusA")
}

func TestParseValueOutOfRange(t *testing.T) {
	src := `package test
type small int8
const (
	smallA small = 100
	smallB small = 200
)
`
	_, err := parseSrc(t, "small", src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "const smallB: value 200 overflows int8")
}

func TestParseUnresolvableValue(t *testing.T) {
	src := `package test

import "math"

type code uint8
const (
	codeA code = 1
	codeB code = math.MaxUint8
)
`
	_, err := parseSrc(t, "code", src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to evaluate value of const codeB")
}

func TestParseCrossFileReference(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.go"), []byte("package test\n\nconst codeBase = 0x100\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "enum.go"), []byte(`package test

type code uint16

const (
	codeA code = codeBase
	codeB code = codeBase + 1
)
`), 0o600))

	gen, err := New("code", dir)
	require.NoError(t, err)
	require.NoError(t, gen.Parse(dir))

	assert.Equal(t, int64(256), constVal(t, gen, "codeA"))
	assert.Equal(t, int64(257), constVal(t, gen, "codeB"))
}
