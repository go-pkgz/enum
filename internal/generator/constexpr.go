package generator

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"strconv"
)

// maxShiftCount caps shift expressions so a malformed source can't ask for an enormous allocation
const maxShiftCount = 512

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

// constDecl is a constant declaration together with the iota value in effect where it appears
type constDecl struct {
	expr    ast.Expr // expression to evaluate, inherited from the previous spec when omitted
	iotaVal int64    // iota value of the spec holding this declaration
}

// constResolver evaluates constant expressions with go/constant, which keeps values exact and covers
// every operator the language allows on integer constants. it holds every constant declared in the
// package so enum values can reference other constants by name.
type constResolver struct {
	decls     map[string]constDecl      // constant name to its defining expression
	typeNames map[string]struct{}       // type names declared in the package, for conversions
	cache     map[string]constant.Value // already resolved constants
	resolving map[string]struct{}       // names being resolved, to detect reference cycles
}

// newConstResolver makes an empty resolver, files are added with addFile
func newConstResolver() *constResolver {
	return &constResolver{
		decls:     map[string]constDecl{},
		typeNames: map[string]struct{}{},
		cache:     map[string]constant.Value{},
		resolving: map[string]struct{}{},
	}
}

// addFile records the constant and type declarations of a single file
func (r *constResolver) addFile(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok {
			return true
		}
		switch decl.Tok {
		case token.TYPE:
			for _, spec := range decl.Specs {
				if tspec, ok := spec.(*ast.TypeSpec); ok {
					r.typeNames[tspec.Name.Name] = struct{}{}
				}
			}
		case token.CONST:
			r.addConstBlock(decl)
		}
		return false
	})
}

// addConstBlock records every constant of a single const block. a spec without an expression repeats
// the expression list of the previous spec, which is how iota based blocks are defined by the language.
func (r *constResolver) addConstBlock(decl *ast.GenDecl) {
	var last []ast.Expr
	for i, spec := range decl.Specs {
		vspec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		if len(vspec.Values) > 0 {
			last = vspec.Values
		}
		for j, name := range vspec.Names {
			if name.Name == "_" || j >= len(last) {
				continue
			}
			// the first declaration wins, a name may be repeated by a block in another scope
			if _, ok := r.decls[name.Name]; ok {
				continue
			}
			r.decls[name.Name] = constDecl{expr: last[j], iotaVal: int64(i)}
		}
	}
}

// resolve evaluates a constant by name
func (r *constResolver) resolve(name string) (constant.Value, error) {
	if v, ok := r.cache[name]; ok {
		return v, nil
	}
	decl, ok := r.decls[name]
	if !ok {
		return nil, fmt.Errorf("unknown constant %s", name)
	}
	if _, ok := r.resolving[name]; ok {
		return nil, fmt.Errorf("constant %s refers to itself", name)
	}
	r.resolving[name] = struct{}{}
	defer delete(r.resolving, name)

	v, err := r.eval(decl.expr, decl.iotaVal)
	if err != nil {
		return nil, fmt.Errorf("constant %s: %w", name, err)
	}
	r.cache[name] = v
	return v, nil
}

// eval evaluates a constant expression with the given iota value
func (r *constResolver) eval(expr ast.Expr, iotaVal int64) (constant.Value, error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return literalValue(e)
	case *ast.Ident:
		if e.Name == "iota" {
			return constant.MakeInt64(iotaVal), nil
		}
		return r.resolve(e.Name)
	case *ast.ParenExpr:
		return r.eval(e.X, iotaVal)
	case *ast.UnaryExpr:
		return r.evalUnary(e, iotaVal)
	case *ast.BinaryExpr:
		return r.evalBinary(e, iotaVal)
	case *ast.CallExpr:
		return r.evalConversion(e, iotaVal)
	}
	return nil, fmt.Errorf("unsupported expression %T", expr)
}

// evalUnary evaluates +x, -x and ^x
func (r *constResolver) evalUnary(e *ast.UnaryExpr, iotaVal int64) (constant.Value, error) {
	x, err := r.eval(e.X, iotaVal)
	if err != nil {
		return nil, err
	}
	if x, err = toInt(x); err != nil {
		return nil, err
	}
	switch e.Op {
	case token.ADD, token.SUB, token.XOR:
		return constant.UnaryOp(e.Op, x, 0), nil
	}
	return nil, fmt.Errorf("unsupported unary operator %s", e.Op)
}

// evalBinary evaluates the arithmetic and bitwise operators defined for integer constants
func (r *constResolver) evalBinary(e *ast.BinaryExpr, iotaVal int64) (constant.Value, error) {
	x, err := r.eval(e.X, iotaVal)
	if err != nil {
		return nil, err
	}
	if x, err = toInt(x); err != nil {
		return nil, err
	}

	if e.Op == token.SHL || e.Op == token.SHR {
		return r.evalShift(e, x, iotaVal)
	}

	y, err := r.eval(e.Y, iotaVal)
	if err != nil {
		return nil, err
	}
	if y, err = toInt(y); err != nil {
		return nil, err
	}

	switch e.Op {
	case token.ADD, token.SUB, token.MUL, token.AND, token.OR, token.XOR, token.AND_NOT:
		return constant.BinaryOp(x, e.Op, y), nil
	case token.QUO, token.REM:
		if constant.Sign(y) == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		if e.Op == token.REM {
			return constant.BinaryOp(x, token.REM, y), nil
		}
		// QUO_ASSIGN keeps the result an integer, plain QUO on two integers yields a rational
		return constant.BinaryOp(x, token.QUO_ASSIGN, y), nil
	}
	return nil, fmt.Errorf("unsupported binary operator %s", e.Op)
}

// evalShift evaluates x << n and x >> n, x is already known to be an integer
func (r *constResolver) evalShift(e *ast.BinaryExpr, x constant.Value, iotaVal int64) (constant.Value, error) {
	y, err := r.eval(e.Y, iotaVal)
	if err != nil {
		return nil, err
	}
	if y, err = toInt(y); err != nil {
		return nil, err
	}
	if constant.Sign(y) < 0 {
		return nil, fmt.Errorf("negative shift count %s", y.ExactString())
	}
	count, exact := constant.Uint64Val(y)
	if !exact || count > maxShiftCount {
		return nil, fmt.Errorf("shift count %s is too large", y.ExactString())
	}
	return constant.Shift(x, e.Op, uint(count)), nil
}

// evalConversion evaluates a single argument conversion such as status(3) or uint8(1 << 2)
func (r *constResolver) evalConversion(e *ast.CallExpr, iotaVal int64) (constant.Value, error) {
	ident, ok := e.Fun.(*ast.Ident)
	if !ok || len(e.Args) != 1 {
		return nil, fmt.Errorf("unsupported call expression")
	}
	_, declared := r.typeNames[ident.Name]
	if _, builtin := intTypes[ident.Name]; !declared && !builtin {
		return nil, fmt.Errorf("unsupported call to %s", ident.Name)
	}
	v, err := r.eval(e.Args[0], iotaVal)
	if err != nil {
		return nil, err
	}
	return toInt(v)
}

// literalValue converts an integer or character literal, covering every base and digit separator
// the language allows
func literalValue(lit *ast.BasicLit) (constant.Value, error) {
	switch lit.Kind {
	case token.INT:
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
	return nil, fmt.Errorf("literal %s is not an integer", lit.Value)
}

// toInt converts a value to an integer, go/constant panics on operands of mismatched kinds so
// everything is checked before it reaches an operator
func toInt(v constant.Value) (constant.Value, error) {
	iv := constant.ToInt(v)
	if iv.Kind() != constant.Int {
		return nil, fmt.Errorf("value %s is not an integer", v.String())
	}
	return iv, nil
}

// checkIntRange reports whether a value fits the underlying type of the enum. an out of range value
// would produce generated code that does not compile. unknown type names are left alone.
func checkIntRange(v constant.Value, underlyingType string) error {
	if underlyingType == "" {
		underlyingType = "int" // the template falls back to int when the type has no explicit underlying type
	}
	info, ok := intTypes[underlyingType]
	if !ok {
		return nil
	}

	if !info.signed {
		if constant.Sign(v) < 0 {
			return fmt.Errorf("value %s is negative but the type is %s", v.ExactString(), underlyingType)
		}
		n, exact := constant.Uint64Val(v)
		if !exact || (info.bits < 64 && n >= uint64(1)<<info.bits) {
			return fmt.Errorf("value %s overflows %s", v.ExactString(), underlyingType)
		}
		return nil
	}

	n, exact := constant.Int64Val(v)
	if !exact {
		return fmt.Errorf("value %s overflows %s", v.ExactString(), underlyingType)
	}
	if info.bits < 64 && (n < -1<<(info.bits-1) || n > 1<<(info.bits-1)-1) {
		return fmt.Errorf("value %s overflows %s", v.ExactString(), underlyingType)
	}
	return nil
}
