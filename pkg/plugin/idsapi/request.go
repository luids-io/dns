// Copyright 2020 Luis Guillén Civera <luisguillenc@gmail.com>. View LICENSE.

package idsapi

import (
	"context"

	"github.com/google/uuid"
)

// defined for context values
type ctxKey int

const (
	keyRequestID ctxKey = 1312
)

// RequestID returns uuid from context. It's unsafe.
func RequestID(ctx context.Context) (uuid.UUID, context.Context, error) {
	svalue := ctx.Value(keyRequestID)
	if svalue == nil {
		v, err := uuid.NewRandom()
		if err != nil {
			return v, ctx, err
		}
		return v, context.WithValue(ctx, keyRequestID, v), nil

	}
	v, _ := svalue.(uuid.UUID)
	return v, ctx, nil
}
