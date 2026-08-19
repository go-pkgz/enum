package generator

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"math/big"
	"strconv"
	"unicode/utf8"
)

// maxShiftLeft caps a left shift so a malformed source can't ask for an enormous allocation. a right
// shift needs no cap, shifting past the width of a value settles at 0 or -1.
const maxShiftLeft = 512

// intTypeInfo describes the width and signedness of a builtin integer type
type intTypeInfo struct {
	bits   int
	signed bool
}

// intTypes maps builtin integer type names to their width. int, uint and uintptr are sized as 64 bit,
// which is the widest they can be on a supported platform.
var intTypes = map[string]intTypeInfo{
	"int":     {bits: 64, signed: true},
	"int8":    {bits: 8, signed: true},
	"int16":   {bits: 16, signed: true},
	"int32":   {bits: 32, signed: true},
	"int64":   {bits: 64, signed: true},
	"rune":    {bits: 32, signed: true},
	"uint":    {bits: 64, signed: false},
	"uint8":   {bits: 8, signed: false},
	"uint16":  {bits: 16, signed: false},
	"uint32":  {bits: 32, signed: false},
	"uint64":  {bits: 64, signed: false},
	"uintptr": {bits: 64, signed: false},
	"byte":    {bits: 8, signed: false},
}

// typedValue is a constant value together with the builtin integer type it carries. the type is empty
// for an untyped constant, it only becomes known through a conversion or a typed declaration, and it
// matters for ^x, which complements within the width of its operand.
type typedValue struct {
	value constant.Value
	typ   string
}

// archSizedUnsigned are the unsigned types whose width is set by the target architecture, which makes
// their complement a different value on a 32 and a 64 bit target
var archSizedUnsigned = map[string]bool{"uint": true, "uintptr": true}

// floatTypes are the builtin floating point types, they take part in constant arithmetic but the
// value of an enum has to come out of it as an integer
var floatTypes = map[string]bool{"float32": true, "float64": true}

// constDecl is a constant declaration together with the iota value in effect where it appears
type constDecl struct {
	expr    ast.Expr // expression to evaluate, inherited from the previous spec when omitted
	typ     string   // builtin type of the declaration, empty when untyped
	iotaVal int64    // iota value of the spec holding this declaration
}

// constResolver evaluates constant expressions with go/constant, which keeps values exact and covers
// every operator the language allows on integer constants. it holds the constants and types declared
// at package level so enum values can refer to them.
type constResolver struct {
	decls     map[string]constDecl  // constant name to its defining expression
	types     map[string]string     // declared type name to the type it is defined as
	cache     map[string]typedValue // already resolved constants
	resolving map[string]struct{}   // names being resolved, to detect reference cycles
}

// newConstResolver makes an empty resolver, files are added with addFile
func newConstResolver() *constResolver {
	return &constResolver{
		decls:     map[string]constDecl{},
		types:     map[string]string{},
		cache:     map[string]typedValue{},
		resolving: map[string]struct{}{},
	}
}

// addFile records the package level constant and type declarations of a single file. declarations
// inside a function are left out, they are not in scope for the enum constants and would shadow the
// package level ones of the same name.
func (r *constResolver) addFile(file *ast.File) {
	for _, d := range file.Decls {
		decl, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch decl.Tok {
		case token.TYPE:
			for _, spec := range decl.Specs {
				tspec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if ident, ok := unparen(tspec.Type).(*ast.Ident); ok {
					r.types[tspec.Name.Name] = ident.Name
				}
			}
		case token.CONST:
			r.addConstBlock(decl)
		}
	}
}

// addConstBlock records every constant of a single const block. a spec without an expression repeats
// the expression list of the previous spec, which is how iota based blocks are defined by the language.
func (r *constResolver) addConstBlock(decl *ast.GenDecl) {
	var last []ast.Expr
	var lastType ast.Expr
	for i, spec := range decl.Specs {
		vspec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		if len(vspec.Values) > 0 {
			last, lastType = vspec.Values, vspec.Type
		}
		typ := ""
		if ident, ok := unparen(lastType).(*ast.Ident); ok {
			typ = ident.Name
		}
		for j, name := range vspec.Names {
			if name.Name == "_" || j >= len(last) {
				continue
			}
			if _, ok := r.decls[name.Name]; ok {
				continue // a name declared twice at package level does not compile, keep the first
			}
			r.decls[name.Name] = constDecl{expr: last[j], typ: typ, iotaVal: int64(i)}
		}
	}
}

// builtinOf resolves a type name to the builtin number type it is defined as, empty when it is not a
// number type or the definition is not visible. a type declared by the package is followed first, it
// shadows a builtin of the same name.
func (r *constResolver) builtinOf(name string) string {
	seen := map[string]struct{}{}
	for name != "" {
		if _, ok := seen[name]; ok {
			return "" // a cycle, which does not compile either
		}
		seen[name] = struct{}{}
		if next, ok := r.types[name]; ok {
			name = next
			continue
		}
		if _, ok := intTypes[name]; ok {
			return name
		}
		if floatTypes[name] || name == "string" {
			return name
		}
		return ""
	}
	return ""
}

// resolve evaluates a constant by name
func (r *constResolver) resolve(name string) (typedValue, error) {
	if v, ok := r.cache[name]; ok {
		return v, nil
	}
	decl, ok := r.decls[name]
	if !ok {
		return typedValue{}, fmt.Errorf("unknown constant %s", name)
	}
	if _, ok := r.resolving[name]; ok {
		return typedValue{}, fmt.Errorf("constant %s refers to itself", name)
	}
	r.resolving[name] = struct{}{}
	defer delete(r.resolving, name)

	v, err := r.eval(decl.expr, decl.iotaVal)
	if err != nil {
		return typedValue{}, fmt.Errorf("constant %s: %w", name, err)
	}
	if typ := r.builtinOf(decl.typ); typ != "" {
		if v, err = r.convert(v, typ); err != nil {
			return typedValue{}, fmt.Errorf("constant %s: %w", name, err)
		}
	}
	r.cache[name] = v
	return v, nil
}

// evalInt evaluates a constant expression and returns its exact integer value
func (r *constResolver) evalInt(expr ast.Expr, iotaVal int64) (constant.Value, error) {
	v, err := r.eval(expr, iotaVal)
	if err != nil {
		return nil, err
	}
	return toInt(v.value)
}

// eval evaluates a constant expression with the given iota value
func (r *constResolver) eval(expr ast.Expr, iotaVal int64) (typedValue, error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		v, err := literalValue(e)
		return typedValue{value: v}, err
	case *ast.Ident:
		if e.Name == "iota" {
			return typedValue{value: constant.MakeInt64(iotaVal)}, nil
		}
		return r.resolve(e.Name)
	case *ast.ParenExpr:
		return r.eval(e.X, iotaVal)
	case *ast.UnaryExpr:
		return r.evalUnary(e, iotaVal)
	case *ast.BinaryExpr:
		return r.evalBinary(e, iotaVal)
	case *ast.CallExpr:
		return r.evalCall(e, iotaVal)
	}
	return typedValue{}, fmt.Errorf("unsupported expression %T", expr)
}

// evalUnary evaluates +x, -x and ^x. the complement of a typed unsigned operand is taken within the
// width of its type, so ^perm(0) of a uint8 based type is 255 rather than -1
func (r *constResolver) evalUnary(e *ast.UnaryExpr, iotaVal int64) (typedValue, error) {
	x, err := r.eval(e.X, iotaVal)
	if err != nil {
		return typedValue{}, err
	}

	switch e.Op {
	case token.ADD, token.SUB:
		if err := requireNumeric(x.value); err != nil {
			return typedValue{}, err
		}
		return typedValue{value: constant.UnaryOp(e.Op, x.value, 0), typ: x.typ}, nil
	case token.XOR:
		v, err := toInt(x.value)
		if err != nil {
			return typedValue{}, err
		}
		prec := 0
		if info, ok := intTypes[x.typ]; ok && !info.signed {
			if archSizedUnsigned[x.typ] {
				// ^uint(0) is 2^32-1 on a 32 bit target and 2^64-1 on a 64 bit one, there is no
				// single value to write into the generated code
				return typedValue{}, fmt.Errorf("complement of %s depends on the target architecture", x.typ)
			}
			prec = info.bits
		}
		return typedValue{value: constant.UnaryOp(e.Op, v, uint(prec)), typ: x.typ}, nil
	}
	return typedValue{}, fmt.Errorf("unsupported unary operator %s", e.Op)
}

// evalBinary evaluates the arithmetic and bitwise operators defined for integer constants
func (r *constResolver) evalBinary(e *ast.BinaryExpr, iotaVal int64) (typedValue, error) {
	x, err := r.eval(e.X, iotaVal)
	if err != nil {
		return typedValue{}, err
	}

	if e.Op == token.SHL || e.Op == token.SHR {
		return r.evalShift(e, x, iotaVal)
	}

	y, err := r.eval(e.Y, iotaVal)
	if err != nil {
		return typedValue{}, err
	}

	if e.Op == token.ADD && x.value.Kind() == constant.String && y.value.Kind() == constant.String {
		return typedValue{value: constant.BinaryOp(x.value, e.Op, y.value)}, nil
	}

	typ := x.typ
	if typ == "" {
		typ = y.typ
	}

	// a typed operand decides the type of the operation, the other one is converted to it. that is
	// what makes x / 2.0 an integer division when x is a typed integer.
	if typ != "" && typ != "string" {
		if x, err = r.convert(x, typ); err != nil {
			return typedValue{}, err
		}
		if y, err = r.convert(y, typ); err != nil {
			return typedValue{}, err
		}
	}

	switch e.Op {
	case token.ADD, token.SUB, token.MUL, token.QUO:
		if err := requireNumeric(x.value); err != nil {
			return typedValue{}, err
		}
		if err := requireNumeric(y.value); err != nil {
			return typedValue{}, err
		}
		op := e.Op
		if e.Op == token.QUO {
			if constant.Sign(y.value) == 0 {
				return typedValue{}, fmt.Errorf("division by zero")
			}
			if x.value.Kind() == constant.Int && y.value.Kind() == constant.Int {
				op = token.QUO_ASSIGN // division of two integers is integer division, plain QUO yields a rational
			}
		}
		v := constant.BinaryOp(x.value, op, y.value)
		if floatTypes[typ] {
			v = roundFloat(v, typ) // a typed float holds every result at the width of its type
		}
		return typedValue{value: v, typ: typ}, nil
	case token.REM, token.AND, token.OR, token.XOR, token.AND_NOT:
		xv, err := toInt(x.value)
		if err != nil {
			return typedValue{}, err
		}
		yv, err := toInt(y.value)
		if err != nil {
			return typedValue{}, err
		}
		if e.Op == token.REM && constant.Sign(yv) == 0 {
			return typedValue{}, fmt.Errorf("division by zero")
		}
		return typedValue{value: constant.BinaryOp(xv, e.Op, yv), typ: typ}, nil
	}
	return typedValue{}, fmt.Errorf("unsupported binary operator %s", e.Op)
}

// evalShift evaluates x << n and x >> n
func (r *constResolver) evalShift(e *ast.BinaryExpr, x typedValue, iotaVal int64) (typedValue, error) {
	xv, err := toInt(x.value)
	if err != nil {
		return typedValue{}, err
	}
	y, err := r.evalInt(e.Y, iotaVal)
	if err != nil {
		return typedValue{}, err
	}
	if constant.Sign(y) < 0 {
		return typedValue{}, fmt.Errorf("negative shift count %s", y.ExactString())
	}
	count, exact := constant.Uint64Val(y)
	switch {
	case e.Op == token.SHL && (!exact || count > maxShiftLeft):
		return typedValue{}, fmt.Errorf("shift count %s is too large", y.ExactString())
	case e.Op == token.SHR:
		// a shift wider than the value itself keeps its result, clamp it to stay in range of uint
		if width := bitLen(xv) + 1; !exact || count > width {
			count = width
		}
	}
	return typedValue{value: constant.Shift(xv, e.Op, uint(count)), typ: x.typ}, nil
}

// evalCall evaluates a conversion such as status(3) or uint8(1 << 2), and the constant builtins
func (r *constResolver) evalCall(e *ast.CallExpr, iotaVal int64) (typedValue, error) {
	ident, ok := unparen(e.Fun).(*ast.Ident)
	if !ok {
		return typedValue{}, fmt.Errorf("unsupported call expression")
	}

	if !r.shadowed(ident.Name) {
		switch ident.Name {
		case "len":
			return r.evalLen(e, iotaVal)
		case "min", "max":
			return r.evalMinMax(e, ident.Name, iotaVal)
		}
	}

	if len(e.Args) != 1 {
		return typedValue{}, fmt.Errorf("unsupported call expression")
	}
	typ := r.builtinOf(ident.Name)
	if typ == "" {
		return typedValue{}, fmt.Errorf("unsupported call to %s", ident.Name)
	}
	v, err := r.eval(e.Args[0], iotaVal)
	if err != nil {
		return typedValue{}, err
	}
	return r.convert(v, typ)
}

// shadowed reports whether the package declares something of its own under a predeclared name
func (r *constResolver) shadowed(name string) bool {
	if _, ok := r.types[name]; ok {
		return true
	}
	_, ok := r.decls[name]
	return ok
}

// evalLen evaluates len of a string constant
func (r *constResolver) evalLen(e *ast.CallExpr, iotaVal int64) (typedValue, error) {
	if len(e.Args) != 1 {
		return typedValue{}, fmt.Errorf("len takes a single argument")
	}
	v, err := r.eval(e.Args[0], iotaVal)
	if err != nil {
		return typedValue{}, err
	}
	if v.value.Kind() != constant.String {
		return typedValue{}, fmt.Errorf("len is only supported for a string constant")
	}
	return typedValue{value: constant.MakeInt64(int64(len(constant.StringVal(v.value))))}, nil
}

// evalMinMax evaluates the min and max builtins over constant arguments
func (r *constResolver) evalMinMax(e *ast.CallExpr, name string, iotaVal int64) (typedValue, error) {
	if len(e.Args) == 0 {
		return typedValue{}, fmt.Errorf("%s takes at least one argument", name)
	}
	op := token.LSS
	if name == "max" {
		op = token.GTR
	}

	var best typedValue
	for i, arg := range e.Args {
		v, err := r.eval(arg, iotaVal)
		if err != nil {
			return typedValue{}, err
		}
		if err := requireNumeric(v.value); err != nil {
			return typedValue{}, err
		}
		if i == 0 || constant.Compare(v.value, op, best.value) {
			best = v
		}
	}
	return best, nil
}

// roundFloat drops the precision a float type cannot hold, the compiler stores a typed float
// constant at the width of its type
func roundFloat(v constant.Value, typ string) constant.Value {
	if typ == "float32" {
		f, _ := constant.Float32Val(v)
		return constant.MakeFloat64(float64(f))
	}
	f, _ := constant.Float64Val(v)
	return constant.MakeFloat64(f)
}

// bitLen is the number of bits an integer value occupies
func bitLen(v constant.Value) uint64 {
	i, ok := constant.Val(v).(*big.Int)
	if !ok {
		return 64 // anything go/constant keeps as an int64
	}
	if n := i.BitLen(); n > 0 {
		return uint64(n)
	}
	return 0
}

// unparen strips the parentheses around an expression
func unparen(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

// convert gives a value the named builtin type, reporting the values it cannot hold
func (r *constResolver) convert(v typedValue, typ string) (typedValue, error) {
	if typ == "string" {
		if v.value.Kind() == constant.Int {
			// converting a number to a string gives the utf-8 encoding of that code point
			n, ok := constant.Int64Val(v.value)
			if !ok || n < 0 || n > utf8.MaxRune {
				n = utf8.RuneError
			}
			return typedValue{value: constant.MakeString(string(rune(n))), typ: typ}, nil
		}
		if v.value.Kind() != constant.String {
			return typedValue{}, fmt.Errorf("value %s is not a string", v.value.String())
		}
		return typedValue{value: v.value, typ: typ}, nil
	}
	if floatTypes[typ] {
		f := constant.ToFloat(v.value)
		if f.Kind() == constant.Unknown {
			return typedValue{}, fmt.Errorf("value %s is not a number", v.value.String())
		}
		return typedValue{value: roundFloat(f, typ), typ: typ}, nil
	}
	iv, err := toInt(v.value)
	if err != nil {
		return typedValue{}, err
	}
	if err := checkIntRange(iv, typ); err != nil {
		return typedValue{}, err
	}
	return typedValue{value: iv, typ: typ}, nil
}

// literalValue converts an integer or character literal, covering every base and digit separator
// the language allows
func literalValue(lit *ast.BasicLit) (constant.Value, error) {
	switch lit.Kind {
	case token.INT, token.FLOAT:
		v := constant.MakeFromLiteral(lit.Value, lit.Kind, 0)
		if v.Kind() == constant.Unknown {
			return nil, fmt.Errorf("invalid literal %s", lit.Value)
		}
		return v, nil
	case token.STRING:
		v := constant.MakeFromLiteral(lit.Value, lit.Kind, 0)
		if v.Kind() == constant.Unknown {
			return nil, fmt.Errorf("invalid literal %s", lit.Value)
		}
		return v, nil
	case token.CHAR:
		// go/constant ignores anything after the first character of a rune literal, unquote it here
		// instead so a literal holding more than one character is rejected
		if len(lit.Value) < 3 || lit.Value[0] != '\'' {
			return nil, fmt.Errorf("invalid literal %s", lit.Value)
		}
		r, _, tail, err := strconv.UnquoteChar(lit.Value[1:], '\'')
		if err != nil || tail != "'" {
			return nil, fmt.Errorf("invalid literal %s", lit.Value)
		}
		return constant.MakeInt64(int64(r)), nil
	}
	return nil, fmt.Errorf("literal %s is not a number", lit.Value)
}

// requireNumeric rejects values arithmetic is not defined for, go/constant panics on operands of
// mismatched kinds so everything is checked before it reaches an operator
func requireNumeric(v constant.Value) error {
	switch v.Kind() {
	case constant.Int, constant.Float:
		return nil
	}
	return fmt.Errorf("value %s is not a number", v.String())
}

// toInt converts a value to an integer, a fractional or non numeric value has no integer form
func toInt(v constant.Value) (constant.Value, error) {
	iv := constant.ToInt(v)
	if iv.Kind() != constant.Int {
		return nil, fmt.Errorf("value %s is not an integer", v.String())
	}
	return iv, nil
}

// checkIntRange reports whether a value fits the given integer type. an out of range value would
// produce generated code that does not compile. unknown type names are left alone.
func checkIntRange(v constant.Value, typ string) error {
	if typ == "" {
		typ = "int" // the template falls back to int when the type has no explicit underlying type
	}
	info, ok := intTypes[typ]
	if !ok {
		return nil
	}

	if !info.signed {
		if constant.Sign(v) < 0 {
			return fmt.Errorf("value %s is negative but the type is %s", v.ExactString(), typ)
		}
		n, exact := constant.Uint64Val(v)
		if !exact || (info.bits < 64 && n >= uint64(1)<<info.bits) {
			return fmt.Errorf("value %s overflows %s", v.ExactString(), typ)
		}
		return nil
	}

	n, exact := constant.Int64Val(v)
	if !exact {
		return fmt.Errorf("value %s overflows %s", v.ExactString(), typ)
	}
	if info.bits < 64 && (n < -1<<(info.bits-1) || n > 1<<(info.bits-1)-1) {
		return fmt.Errorf("value %s overflows %s", v.ExactString(), typ)
	}
	return nil
}
