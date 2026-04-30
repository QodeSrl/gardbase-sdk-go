package gardb

import "github.com/QodeSrl/gardbase/pkg/api/objects"

type QueryCondition struct {
	Op    objects.QueryOperator
	Value any
}

func Eq(val any) QueryCondition {
	return QueryCondition{Op: objects.QueryEq, Value: val}
}

func Lt(val any) QueryCondition {
	return QueryCondition{Op: objects.RangeLt, Value: val}
}

func Lte(val any) QueryCondition {
	return QueryCondition{Op: objects.RangeLte, Value: val}
}

func Gt(val any) QueryCondition {
	return QueryCondition{Op: objects.RangeGt, Value: val}
}

func Gte(val any) QueryCondition {
	return QueryCondition{Op: objects.RangeGte, Value: val}
}

func Between(val1, val2 any) QueryCondition {
	return QueryCondition{Op: objects.RangeBetween, Value: [2]any{val1, val2}}
}
