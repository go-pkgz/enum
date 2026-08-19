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

// resolveSrc parses a go source and resolves a single constant from it
func resolveSrc(t *testing.T, src, name string) (constant.Value, error) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "src.go", src, parser.ParseComments)
	require.NoError(t, err)
	r := newConstResolver()
	r.addFile(file)
	return r.resolve(name)
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
		{"float literal", "const x = 3.14", "not an integer"},
		{"float expression", "const x = 1.5 + 1.5", "not an integer"},
		{"unknown constant", "const x = missing + 1", "unknown constant missing"},
		{"package reference", "const x = math.MaxInt8", "unsupported expression"},
		{"function call", `const x = len("ab")`, "unsupported call to len"},
		{"comparison operator", "const x = 1 < 2", "unsupported binary operator"},
		{"self reference", "const x = x + 1", "refers to itself"},
		{"reference cycle", "const (\n\tx = y\n\ty = x\n)", "refers to itself"},
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

	_, err = literalValue(&ast.BasicLit{Kind: token.STRING, Value: `"str"`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an integer")

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
