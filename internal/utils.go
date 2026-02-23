package internal

import (
	"context"
	"errors"

	"github.com/QodeSrl/gardbase/pkg/api/objects"
)

func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

type Index struct {
	Name       objects.IndexName
	HashValue  any
	RangeValue any
}
