package ast

import (
	"fmt"
	"strings"

	"github.com/project-flogo/core/data"
	"github.com/project-flogo/core/data/coerce"
	"github.com/project-flogo/core/data/resolve"
)

type Expr interface {
	Init(resolver resolve.CompositeResolver, root bool) error //todo can use root to multi-thread eval of root node

	Eval(scope data.Scope) (interface{}, error)
}

func evalLR(left, right Expr, scope data.Scope) (lv, rv interface{}, err error) {
	lv, err = left.Eval(scope)
	if err != nil {
		return nil, nil, err
	}
	rv, err = right.Eval(scope)
	return lv, rv, err
}

func NewExprList(x interface{}) ([]Expr, error) {
	if x, ok := x.(Expr); ok {
		return []Expr{x}, nil
	}
	return nil, fmt.Errorf("invalid expression list expression type; expected ast.Expr, got %T", x)
}

func AppendToExprList(list, x interface{}) ([]Expr, error) {
	lst, ok := list.([]Expr)
	if !ok {
		return nil, fmt.Errorf("invalid expression list type; expected []ast.Expr, got %T", list)
	}
	if x, ok := x.(Expr); ok {
		return append(lst, x), nil
	}
	return nil, fmt.Errorf("invalid expression list expression type; expected ast.Expr, got %T", x)
}

func NewTernaryExpr(ifNode, thenNode, elseNode interface{}) (Expr, error) {

	ifExpr := ifNode.(Expr)
	thenExpr := thenNode.(Expr)
	elseExpr := elseNode.(Expr)

	return &exprTernary{ifExpr: ifExpr, thenExpr: thenExpr, elseExpr: elseExpr}, nil
}

type exprTernary struct {
	ifExpr, thenExpr, elseExpr Expr
}

func (e *exprTernary) Init(resolver resolve.CompositeResolver, root bool) error {
	err := e.ifExpr.Init(resolver, false)
	if err != nil {
		return err
	}
	err = e.thenExpr.Init(resolver, false)
	if err != nil {
		return err
	}
	err = e.elseExpr.Init(resolver, false)
	return err
}

func (e *exprTernary) Eval(scope data.Scope) (interface{}, error) {

	iv, err := e.ifExpr.Eval(scope)
	if err != nil {
		return nil, err
	}

	bv, err := coerce.ToBool(iv)
	if err != nil {
		return nil, err
	}

	if bv {
		tv, err := e.thenExpr.Eval(scope)
		if err != nil {
			return nil, err
		}
		return tv, nil
	} else {
		ev, err := e.elseExpr.Eval(scope)
		if err != nil {
			return nil, err
		}
		return ev, nil
	}
}

func NewRefExpr(refNode ...interface{}) (Expr, error) {
	expr, err := Concat(refNode...)
	if err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(string(expr.Lit)) //todo is trim overkill
	return &exprRef{ref: ref}, nil
}

type exprRef struct {
	ref string
	res resolve.Resolution
}

func (e *exprRef) Init(resolver resolve.CompositeResolver, root bool) error {

	r, err := resolver.GetResolution(e.ref)
	if err != nil {
		return err
	}

	e.res = r
	return nil
}

func (e *exprRef) Eval(scope data.Scope) (interface{}, error) {
	return e.res.GetValue(scope)
}
