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

// RequestID returns uuid from context, it creates if doesn't exist. It's unsafe.
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

// SetRequestID sets a new uuid in context.
func SetRequestID(ctx context.Context, newuid uuid.UUID) context.Context {
	return context.WithValue(ctx, keyRequestID, newuid)
}

// GetRequestID returns uuid from context.
func GetRequestID(ctx context.Context) uuid.UUID {
	svalue := ctx.Value(keyRequestID)
	if svalue == nil {
		return uuid.Nil
	}
	v, _ := svalue.(uuid.UUID)
	return v
}
