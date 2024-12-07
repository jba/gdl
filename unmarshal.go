// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

package gdl

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// UnmarshalValues unmarshals a list of Values into a pointer to a struct.
// The struct's fields should all be slices of struct type.
// The first word of each Value selects the field; see [UnmarshalValue] for
// details.
func UnmarshalValues(vals []Value, p any) (err error) {
	rv := reflect.ValueOf(p)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("gdl.UnmarshalValues: second argument must be pointer to struct, not %T", p)
	}
	rv = rv.Elem()

	for _, val := range vals {
		if err := unmarshalValue(val, rv); err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalValue unmarshals a [Value] v into a pointer to a struct.
// Each field of the struct should be a scalar type (integer, float, bool or string),
// A slice of scalars, or a slice of structs.
//
// The scalar fields are populated with the words of v in order.
// If the final field is a slice of scalars, it is set to the remaining words.
// For example, unmarshaling this value:
//
//	17 hello big world
//
// into a value of this type:
//
//	type Thing struct { A int; B string; C []string}
//
// results in this value:
//
//	Thing{A: 17, B: "hello", C: []string{"big", "world"}}
//
// After scalar fields, the struct can contain slices of structs.
// The first word after the scalar fields are matched selects the field,
// and the remaining words are unmarshaled into a value of that field's type and appended to it.
// For example, add this type definition to that of S above:
//
//	type S struct { Things []Thing}
//
// Then unmarshaling this value:
//
//	thing 17 hello big world
//
// into a value of type S results in
//
//	S{
//	    Things: {{A: 17, B: "hello", C: []string{"big", "world"}}},
//	}
//
// For slices of structs, the field name is matched with a word as follows:
// The match can be exact, or with the first rune lower-cased, or pluralized.
// The plural rules are very simple: if the word ends in 's' or 'x', then
// "es" is appended to it; otherwise "s" is appended.
// For example, a field named "Things" will match the following words:
//
//	Things
//	Thing
//	things
//	thing
func UnmarshalValue(v Value, p any) error {
	rv := reflect.ValueOf(p)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("gdl.UnmarshalValue: second argument must be pointer to struct, not %T", p)
	}
	return unmarshalValue(v, rv.Elem())
}

func unmarshalValue(v Value, rv reflect.Value) error {
	t := rv.Type()
	if t.Kind() != reflect.Struct {
		panic("expected struct")
	}
	prog, err := programFor(t)
	if err != nil {
		return fmt.Errorf("%s:%d: %w", v.File, v.Line, err)
	}
	if err := prog.run(rv, v.Words); err != nil {
		return fmt.Errorf("%s:%d: %w", v.File, v.Line, err)
	}
	return nil
}

var programs sync.Map // reflect.Type to *program

func programFor(t reflect.Type) (*program, error) {
	if prog, ok := programs.Load(t); ok {
		return prog.(*program), nil
	}
	// We don't need locking, all programs for a type are identical.
	prog, err := compile(t)
	if err != nil {
		return nil, err
	}
	programs.Store(t, prog)
	return prog, nil
}

// program is a program for setting values of a type from a slice of strings.
type program struct {
	t       reflect.Type
	idIndex []int      // index of ID field; group by first word
	ops     map[any]op // key is integer index or word
}

type op func(reflect.Value, []string) ([]string, error)

// s is a struct. words is from a Value, positioned just after the first word.
func (p *program) run(rv reflect.Value, words []string) error {
	var err error
	ws := words
	for len(ws) > 0 {
		i := len(words) - len(ws)
		op, byIndex := p.findOp(i, ws[0])
		if op == nil {
			return fmt.Errorf("could not set %q at index %d into value of type %s, words=%v",
				ws[0], i, rv.Type(), words)
		}
		if !byIndex {
			ws = ws[1:]
		}
		ws, err = op(rv, ws)
		if err != nil {
			return err
		}
	}
	return nil
}

// bool is whether it matched on index.
func (p *program) findOp(i int, w string) (op, bool) {
	if op, ok := p.ops[i]; ok {
		return op, true
	}
	w = lowerFirst(w)
	if op, ok := p.ops[w]; ok {
		return op, false
	}
	if op, ok := p.ops[plural(w)]; ok {
		return op, false
	}
	return nil, false
}

// t must be a struct type.
func compile(t reflect.Type) (*program, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", t)
	}
	sfs := reflect.VisibleFields(t)
	p := &program{
		t:   t,
		ops: map[any]op{},
	}
	ii, err := idIndex(sfs)
	if err != nil {
		return nil, err
	}
	p.idIndex = ii
	// The ID field is set outside the program for this type, so, skip it.
	// It is always the first field.
	if ii != nil {
		sfs = sfs[1:]
	}
	for i, sf := range sfs {
		setf := setScalarFunc(sf.Type)
		if setf != nil {
			// sf is of scalar type: it matches by position.
			op := func(rv reflect.Value, words []string) ([]string, error) {
				fv, err := rv.FieldByIndexErr(sf.Index)
				if err != nil {
					// TODO: create the nil pointers.
					return nil, err
				}
				return words[1:], setf(fv, words[0])
			}
			p.ops[i] = op
		} else {
			switch sf.Type.Kind() {
			case reflect.Slice:
				elemType := sf.Type.Elem()
				setf := setScalarFunc(elemType)
				if setf != nil {
					// sf is a slice of scalars: it takes the rest of the words.
					// TODO: check that there are no fields in this struct that use the word
					// as as index (that is, fields of non-scalar slice type).
					if i+1 != len(sfs) {
						return nil, fmt.Errorf("scalar slice field %s must be last field in struct %s",
							sf.Name, t)
					}
					op := func(rv reflect.Value, words []string) ([]string, error) {
						fv, err := rv.FieldByIndexErr(sf.Index)
						if err != nil {
							// TODO: create the nil pointers.
							return nil, err
						}
						for _, w := range words {
							fv.Set(reflect.Append(fv, reflect.Zero(fv.Type().Elem())))
							if err := setf(fv.Index(fv.Len()-1), w); err != nil {
								return nil, err
							}
						}
						return nil, nil
					}
					p.ops[i] = op
				} else {
					// A slice of non-scalar type: match on field name.
					if elemType.Kind() == reflect.Pointer {
						elemType = elemType.Elem()
					}
					subprog, err := programFor(elemType)
					if err != nil {
						return nil, err
					}
					// Matching word has been removed before being passed to this function.
					op := func(rv reflect.Value, words []string) ([]string, error) {
						fv, err := rv.FieldByIndexErr(sf.Index)
						if err != nil {
							// TODO: create the nil pointers.
							return nil, err
						}
						var elem reflect.Value
						if subprog.idIndex != nil {
							if len(words) == 0 {
								return nil, errors.New("no words for struct with ID")
							}
							for i := 0; i < fv.Len(); i++ {
								elem = fv.Index(i)
								idf, err := elem.FieldByIndexErr(subprog.idIndex)
								if err != nil {
									return nil, err
								}
								if idf.Interface() == words[0] {
									break
								}
							}
							if !elem.IsValid() {
								fv.Set(reflect.Append(fv, reflect.Zero(fv.Type().Elem())))
								elem = fv.Index(fv.Len() - 1)
								idf, err := elem.FieldByIndexErr(subprog.idIndex)
								if err != nil {
									return nil, err
								}
								idf.SetString(words[0])
							}
							words = words[1:]
						} else {
							fv.Set(reflect.Append(fv, reflect.Zero(fv.Type().Elem())))
							elem = fv.Index(fv.Len() - 1)
						}
						return nil, subprog.run(elem, words)
					}
					p.ops[sf.Name] = op
					p.ops[lowerFirst(sf.Name)] = op
				}
			}
		}
	}
	return p, nil
}

func idIndex(sfs []reflect.StructField) ([]int, error) {
	if len(sfs) == 0 {
		return nil, nil
	}
	f0 := sfs[0]
	tag := f0.Tag.Get("gdl")
	if tag == "" {
		return nil, nil
	}
	_, rest, _ := strings.Cut(tag, ",")
	if strings.TrimSpace(rest) != "id" {
		return nil, nil
	}
	if f0.Type.Kind() != reflect.String {
		return nil, fmt.Errorf("ID field %s must be a string", f0.Name)
	}
	return f0.Index, nil
}

func plural(s string) string {
	if len(s) == 0 {
		return s
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	switch r {
	case 's', 'x':
		return s + "es"
	default:
		return s + "s"
	}
}
func lowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	rs := []rune(s)
	rs[0] = unicode.ToLower(rs[0])
	return string(rs)
}

func setScalarFunc(t reflect.Type) func(reflect.Value, string) error {
	switch t.Kind() {
	case reflect.String:
		return func(rv reflect.Value, s string) error {
			rv.SetString(s)
			return nil
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func(rv reflect.Value, s string) error {
			i, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return err
			}
			rv.SetInt(i)
			return nil
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return func(rv reflect.Value, s string) error {
			i, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return err
			}
			rv.SetUint(i)
			return nil
		}

	case reflect.Float32, reflect.Float64:
		return func(rv reflect.Value, s string) error {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return err
			}
			rv.SetFloat(f)
			return nil
		}

	case reflect.Bool:
		return func(rv reflect.Value, s string) error {
			b, err := strconv.ParseBool(s)
			if err != nil {
				return err
			}
			rv.SetBool(b)
			return nil
		}
	default:
		return nil
	}
}

// func UnmarshalValues(vals []Value, p any) (err error) {
// 	rv := reflect.ValueOf(p)
// 	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
// 		return fmt.Errorf("gdl.UnmarshalValues: second argument must be pointer to struct, not %T", p)
// 	}
// 	rv = rv.Elem()

// 	defer func() {
// 		if err != nil {
// 			err = fmt.Errorf("gdl.UnmarshalValues into %s: %w", rv.Type(), err)
// 		}
// 	}()

// 	sfs := reflect.VisibleFields(rv.Type())
// 	seen := map[string]bool{}
// 	for val := range vals {
// 		if len(val.Head) == 0 {
// 			return fmt.Errorf("%s: value has no head", val.Pos())
// 		}
// 		if len(val.Head[0]) == 0 {
// 			return fmt.Errorf("%s: value has empty head", val.Pos())
// 		}
// 		key := val.Head[0]
// 		sf, ok := matchField(key, sfs)
// 		if !ok {
// 			return fmt.Errorf("%s: no field in struct matches %q", val.Pos(), key)
// 		}
// 		fv, err := rv.FieldByIndexErr(sf.Index)
// 		if err != nil {
// 			// TODO: create the nil pointers.
// 			return err
// 		}
// 		if fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array {
// 			fv.Set(reflect.Append(fv, reflect.Zero(fv.Type().Elem())))
// 			if err := unmarshalReflectValue(val, fv.Index(fv.Len()-1)); err != nil {
// 				return err
// 			}
// 		} else {
// 			if seen[key] {
// 				return fmt.Errorf("%s: key %q occurs more than once", val.Pos(), key)
// 			}
// 			seen[key] = true
// 			if err := unmarshalReflectValue(val, fv); err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }
