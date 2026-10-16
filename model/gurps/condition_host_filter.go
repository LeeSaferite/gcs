// Copyright (c) 1998-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package gurps

import (
	"context"
	"strings"

	"github.com/richardwilkes/gcs/v5/model/condition"
	"github.com/richardwilkes/gcs/v5/model/fxp"
)

var _ condition.Host = &filterHost[*Trait]{}

// filterHost is the condition.Host for list filters over rows of type T. It reads the fields through the accessors of
// a filter field table. A blank text field, an empty list field and a field the item lacks, as the field's Has says,
// are Absent, which meets no criterion, as legacy filters treat them; it has no entity.
type filterHost[T Node[T]] struct {
	fields map[string]*FilterField[T]
}

func newFilterHost[T Node[T]](fields []*FilterField[T]) *filterHost[T] {
	h := &filterHost[T]{fields: make(map[string]*FilterField[T], len(fields))}
	for _, f := range fields {
		h.fields[f.Key] = f
	}
	return h
}

// Collection implements condition.Host. A filter has no collections.
func (h *filterHost[T]) Collection(_ string, _, _ condition.Item) ([]condition.Item, bool) {
	return nil, false
}

// Field implements condition.Host.
func (h *filterHost[T]) Field(item condition.Item, key string) (condition.Value, bool) {
	fn, ok := h.FieldFunc("", key)
	if !ok {
		return condition.Value{}, false
	}
	return fn(item)
}

// FieldFunc implements condition.FieldResolver, so a Matcher looks each field's accessor up once.
func (h *filterHost[T]) FieldFunc(_, key string) (func(condition.Item) (condition.Value, bool), bool) {
	f, known := h.fields[key]
	if !known {
		return nil, false
	}
	var value func(row T) condition.Value
	switch f.Kind {
	case FilterFieldText:
		value = func(row T) condition.Value {
			if text := f.Text(row); strings.TrimSpace(text) != "" {
				return textValue(text)
			}
			return condition.Value{}
		}
	case FilterFieldList:
		value = func(row T) condition.Value { return listValue(f.List(row)) }
	case FilterFieldNumber:
		value = func(row T) condition.Value { return numberValue(f.Number(row)) }
	case FilterFieldWeight:
		value = func(row T) condition.Value {
			return condition.Value{Kind: condition.ValueWeight, Number: fxp.Int(f.Weight(row))}
		}
	case FilterFieldBool:
		value = func(row T) condition.Value { return boolValue(f.Bool(row)) }
	default:
		return nil, false
	}
	return func(item condition.Item) (condition.Value, bool) {
		row, ok := item.(T)
		switch {
		case !ok:
			return condition.Value{}, false
		case f.Has != nil && !f.Has(row):
			return condition.Value{}, true
		default:
			return value(row), true
		}
	}, true
}

// Attribute implements condition.Host. A filter has no attributes.
func (h *filterHost[T]) Attribute(_ string) (fxp.Int, bool) {
	return 0, false
}

// Entity implements condition.Host. A filter has no entity.
func (h *filterHost[T]) Entity(_ string) (fxp.Int, bool) {
	return 0, false
}

// Replacements implements condition.Host. Legacy filters apply no replacements.
func (h *filterHost[T]) Replacements(_, _ condition.Item) map[string]string {
	return nil
}

// RunScript implements condition.Host. Filters have no scripts.
func (h *filterHost[T]) RunScript(_ context.Context, _ *condition.Script, _, _ condition.Item) (state condition.State, reason string) {
	return condition.Failed, ""
}
