// Copyright 2023 Olivier Mengué
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package regexpstruct extends [regexp] to store submatches into structs.
//
// [Regexp] is a generic type that extends [regexp.Regexp] to provide additional
// methods that store capture results into a given struct, matching struct tags
// with captures names.
//
// The following methods are exposed:
//   - [Regexp.FindStringStruct]: similar to [regexp.FindStringSubmatch]
//   - [Regexp.FindAllStringStruct]: similar to [regexp.FindAllStringSubmatch]
package regexpstruct

import (
	"fmt"
	"reflect"
	"regexp"
)

// re is defined only for private embedding
type re = *regexp.Regexp

// Regexp extends [regexp.Regexp] with methods allowing to store captures into struct T.
//
// All the [regexp.Regexp] methods are available.
type Regexp[T any] struct {
	re
	captures []capture
}

type capture struct {
	index int
	get   func(reflect.Value) reflect.Value
}

// Compile wraps [regexp.Compile] to extend [regexp.Regexp] as [Regexp].
//
// Type T must be a struct type with struct tags structTag that must match
// names of submatches of the regexp. Submatches names are either integers or
// defined using the capturing group (?P<name>re) (see [regexp/syntax]) and are exposed by
// [regexp.Regexp.SubexpNames].
// See also [regexp.Regexp.Expand] for capture naming constraints.
//
// Recommended tag names: "re", "rx", or "regexp".
func Compile[T any](expr string, structTag string) (*Regexp[T], error) {
	if structTag == "" {
		panic("invalid tag name")
	}
	if reflect.TypeOf((*T)(nil)).Elem().Kind() != reflect.Struct {
		panic("T must be a struct type")
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	matchesNames := re.SubexpNames()

	fields := extractFields(reflect.TypeOf((*T)(nil)).Elem(), structTag)
	if len(fields) == 0 {
		var zeroT T
		panic(fmt.Errorf("type %T has no fields with stuct tag %q", zeroT, structTag))
	}

	captures := make([]capture, 0, len(matchesNames))
	for i := 1; i < len(matchesNames); i++ {
		name := matchesNames[i]
		if name == "" {
			continue
		}
		if get := fields[name]; get != nil {
			captures = append(captures, capture{index: i, get: get})
		}
	}

	return &Regexp[T]{
		re:       re,
		captures: captures,
	}, nil
}

// MustCompile is like Compile but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled
// regular expressions.
func MustCompile[T any](expr string, structTag string) *Regexp[T] {
	re, err := Compile[T](expr, structTag)
	if err != nil {
		panic(err)
	}
	return re
}

var (
	typeEmptyStruct     = reflect.TypeOf(struct{}{})
	typeSetter          = reflect.TypeOf((*interface{ Set(string) error })(nil)).Elem()
	typeTextUnmarshaler = reflect.TypeOf((*interface{ UnmarshalText([]byte) error })(nil)).Elem()
)

func extractFields(t reflect.Type, tagName string) (fields map[string]func(reflect.Value) reflect.Value) {
	switch t.Kind() {
	case reflect.Ptr:
		fields = extractFields(t.Elem(), tagName)
		wrapFields(fields,
			func(v reflect.Value) reflect.Value {
				if v.IsNil() {
					v.Set(reflect.New(v.Type().Elem()))
				}
				return v.Elem()
			})
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			index := i
			f := t.Field(index)
			if tag, ok := f.Tag.Lookup(tagName); ok && tag != "" {
				if fields == nil {
					fields = make(map[string]func(reflect.Value) reflect.Value)
				}

				/*
					typeName := f.Type.Name()
					isSetter := f.Type.AssignableTo(typeSetter)
					isUnmarshaler := f.Type.AssignableTo(typeTextUnmarshaler)
					_, _, _ = typeName, isSetter, isUnmarshaler
				*/

				isStruct := f.Type.Kind() == reflect.Struct &&
					(f.Type.Name() == "" ||
						(!f.Type.AssignableTo(typeSetter) && !f.Type.AssignableTo(typeTextUnmarshaler)))
				if isStruct {
					fields2 := extractFields(f.Type, tagName)
					for name, g := range fields2 {
						getter := g
						fields[tag+"__"+name] = func(v reflect.Value) reflect.Value { return getter(v.Field(index)) }
					}
				} else {
					fields[tag] = func(v reflect.Value) reflect.Value { return v.Field(index) }
				}
			} else if f.Anonymous { // recurse into embedded struct
				fields2 := extractFields(f.Type, tagName)
				wrapFields(fields2, func(v reflect.Value) reflect.Value { return v.Field(index) })
				if fields == nil {
					fields = fields2
				} else {
					for name, getter := range fields2 {
						fields[name] = getter
					}
				}
			}
		}
	default: // ignore
	}
	return
}

func wrapFields(fields map[string]func(reflect.Value) reflect.Value, w func(reflect.Value) reflect.Value) {
	for name := range fields {
		inner := fields[name]
		fields[name] = func(v reflect.Value) reflect.Value { return inner(w(v)) }
	}
}

func deserialize(matches []string, captures []capture, target reflect.Value) {
	for _, m := range captures {
		m.get(target).SetString(matches[m.index])
	}
}

// FindStringStruct wraps [regexp.Regexp.FindStringSubmatch] to store submatches into
// a struct type value using struct tags.
func (re *Regexp[T]) FindStringStruct(s string, target *T) bool {
	matches := re.re.FindStringSubmatch(s)
	if matches == nil {
		return false
	}
	deserialize(matches, re.captures, reflect.ValueOf(target).Elem())
	return true
}

// FindAllStringStruct wraps [regexp.Regexp.FinfAllStringSubmatch] to store repeated
// captures a into a []T.
func (re *Regexp[T]) FindAllStringStruct(s string, n int) []T {
	matches := re.re.FindAllStringSubmatch(s, n)
	if matches == nil {
		return nil
	}
	nbMatches := len(matches)

	r := make([]T, nbMatches)
	v := reflect.ValueOf(r)
	for i := 0; i < nbMatches; i++ {
		deserialize(matches[i], re.captures, v.Index(i))
	}
	return r
}
