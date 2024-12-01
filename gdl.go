// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: support unmarshaling into any.

// TODO: rules on reuse:
// If you re-use a data structure for unmarshaling, beware:
// - You may have a value set that does not appear in the second input; it will still be there.
// - That goes for struct fields, array/slice elements, map items.
// (Consider a "clear" mode that zeros these things and sets slice lengths; but
// maybe it's not worth it.)
// (It certainly doesn't make sense to do this for slices/arrays but not for struct fields; that
// would just be inconsistent. All or nothing.)

// gdl implements a decription language based on the file format of go.mod
// and similar files.
// The format is line-oriented.
// If the last non-whitespace character on a line is a backslash, the line continues
// onto the next line.
// Comments begin with // and extend to the end of the line.
// Backslashes are ignored inside a comment.
//
// Each line is a sequence of words separated by whitespace.
// A word can include whitespace by enclosing it in double quotes or backticks.
// Both kinds of quotations are interpreted according to Go syntax.

// An open parenthesis at the end of a line starts a sequence, which ends on a line
// consisting only of a close parenthesis.
package gdl

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Value struct {
	Head []string
	List []Value
}

// ParseFile returns a sequence of Values parsed from the contents of the file.
// If there is an error, the sequence stops and the error function will return the error.
func FileValues(filename string) (iter.Seq[Value], func() error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return func(func(Value) bool) { return }, func() error { return err }
	}
	return parse(string(data), filename)
}

// Values returns a sequence of Values parsed from s.
// If there is an error, the sequence stops and the error function will return the error.
func Values(s string) (iter.Seq[Value], func() error) {
	return parse(s, "<no file>")
}

// Parse parses a single Value from s.
// It returns an error if the string contains no Value, or more than one Value.
func Parse(s string) (Value, error) {
	vals, errf := parse(s, "<no file>")
	next, stop := iter.Pull(vals)
	defer stop()
	val, ok := next()
	if !ok {
		return Value{}, cmp.Or(errf(), fmt.Errorf("gdl.Parse: no value in %q", s))
	}
	if _, ok = next(); ok {
		return Value{}, fmt.Errorf("gdl.Parse: more than one value in %q", s)
	}
	if err := errf(); err != nil {
		return Value{}, err
	}
	return val, nil
}

// parse parses s into a sequence of Values. The filename is only for display in errors.
func parse(s, filename string) (iter.Seq[Value], func() error) {
	var err error
	iter := func(yield func(Value) bool) {
		lex := newLexer(s)
		for {
			tok := skipNewlines(lex)
			switch tok.kind {
			case tokEOF:
				return
			case ')':
				err = fmt.Errorf("%s:%d: unexpected close paren", filename, lex.lineno)
				return
			default:
				val, e := parseValue(tok, lex)
				if e != nil {
					e = fmt.Errorf("%s:%d: %w", filename, lex.lineno, e)
					return
				}
				if !yield(val) {
					return
				}
			}
		}
	}
	return iter, func() error { return err }
}

// Called at line start. Ends at the next line start or EOF.
// Only called when there is a value.
func parseValue(tok token, lex *lexer) (Value, error) {
	var head []string
	for {
		switch tok.kind {
		case tokEOF:
			// Accept a value that isn't followed by a newline.
			if len(head) > 0 {
				return Value{Head: head}, nil
			}
			return Value{}, io.ErrUnexpectedEOF

		case '\n':
			if len(head) > 0 {
				return Value{Head: head}, nil
			}
			return Value{}, errors.New("unexpected newline")
			// // blank line
			// continue

		case tokWord:
			head = append(head, tok.val)

		case tokString:
			unq, err := strconv.Unquote(tok.val)
			if err != nil {
				return Value{}, err
			}
			head = append(head, unq)

		case '(':
			list, err := parseParenList(lex)
			if err != nil {
				return Value{}, err
			}
			return Value{Head: head, List: list}, nil

		case ')':
			lex.unget(tok)
			if len(head) == 0 {
				panic("bad close paren")
			}
			return Value{Head: head}, nil

		case tokErr:
			return Value{}, tok.err

		default:
			panic("bad token kind")
		}
		tok = lex.next()
	}
}

// Called just after '('. Ends at start of line.
func parseParenList(lex *lexer) ([]Value, error) {
	var vs []Value
	for {
		tok := skipNewlines(lex)
		switch tok.kind {
		case tokEOF:
			return nil, io.ErrUnexpectedEOF
		case ')':
			k := lex.peek()
			switch k {
			case tokErr:
				return nil, lex.next().err
			case ')', '\n', tokEOF:
				return vs, nil
			default:
				return nil, errors.New("close paren must be followed by newline, EOF or another close paren")
			}
		}
		v, err := parseValue(tok, lex)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
}

func skipNewlines(lex *lexer) token {
	for {
		tok := lex.next()
		if tok.kind != '\n' {
			return tok
		}
	}
}

// TODO: support head where the list has multiple values, like "require ( m1 v1; m2 v2)".

// Unmarshal values takes a sequence of Values and unmarshals them into p.
// p must be a pointer to a struct.
// Each Value in the sequence must have a non-empty head. The first head value
// must match a field in the struct.
// If that field has a slice type, the value is unmarshaled and appended to the slice.
// Otherwise, the first head value must be unique in the sequence. It is unmarshaled
// into the field.
//
// The head is matched against the exported fields of the struct case-insensitively.
// If a field name doesn't match, the match is retried with the plural of head.
// The plural rules are very simple: if the word ends in 's' or 'x', then
// "es" is appended to it; otherwise "s" is appended.
func UnmarshalValues(vals iter.Seq[Value], p any) (err error) {
	rv := reflect.ValueOf(p)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("gdl.UnmarshalValues: second argument must be pointer to struct, not %T", p)
	}
	rv = rv.Elem()

	defer func() {
		if err != nil {
			err = fmt.Errorf("gdl.UnmarshalValues into %s: %w", rv.Type(), err)
		}
	}()

	sfs := reflect.VisibleFields(rv.Type())
	seen := map[string]bool{}
	for val := range vals {
		if len(val.Head) == 0 {
			return fmt.Errorf("value %+v has no head", val)
		}
		if len(val.Head[0]) == 0 {
			return fmt.Errorf("value %+v has empty head", val)
		}
		key := val.Head[0]
		sf, ok := matchField(key, sfs)
		if !ok {
			return fmt.Errorf("no field in struct matches %q", key)
		}
		fv, err := rv.FieldByIndexErr(sf.Index)
		if err != nil {
			// TODO: create the nil pointers.
			return err
		}
		if fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array {
			fv.Set(reflect.Append(fv, reflect.Zero(fv.Type().Elem())))
			if err := unmarshalReflectValue(val, fv.Index(fv.Len()-1)); err != nil {
				return err
			}
		} else {
			if seen[key] {
				return fmt.Errorf("key %q occurs more than once", key)
			}
			seen[key] = true
			if err := unmarshalReflectValue(val, fv); err != nil {
				return err
			}
		}
	}
	return nil
}

// TODO: support struct tags here.

func matchField(s string, fields []reflect.StructField) (reflect.StructField, bool) {
	var ps string
	r, _ := utf8.DecodeLastRuneInString(s)
	switch r {
	case 's', 'x':
		ps = s + "es"
	default:
		ps = s + "s"
	}
	for _, sf := range fields {
		if strings.EqualFold(s, sf.Name) || strings.EqualFold(ps, sf.Name) {
			return sf, true
		}
	}
	return reflect.StructField{}, false
}

func UnmarshalValue(v Value, p any) error {
	return unmarshalReflectValue(v, reflect.ValueOf(p))
}

func unmarshalReflectValue(v Value, rv reflect.Value) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("gdl.UnmarshalValue into %s: %w", rv.Type(), err)
		}
	}()

	switch rv.Kind() {

	case reflect.Slice, reflect.Array:
		// Don't clear or set length unless we want to do it for everything, even struct fields.
		// Unmarshal either the head or the list. The other must be empty.
		// TODO: Can json.Unmarshal take a straight slice, or does it need ptr to slice?
		// If head is empty, unmarshal each value into an element.
		if len(v.Head) == 0 {
			return unmarshalSliceOrArray(v.List, rv)
		}
		if len(v.List) == 0 {
			// TODO: optimize.
			var vs []Value
			for _, h := range v.Head {
				vs = append(vs, Value{Head: []string{h}})
			}
			return unmarshalSliceOrArray(vs, rv)
		}
		return errors.New("cannot unmarshal a value that has both a head and a list into a slice or array")

	case reflect.Map:
		return unmarshalMap(v, rv)

	case reflect.Pointer:
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		return unmarshalReflectValue(v, rv.Elem())
	case reflect.Interface:
		// Like pointer?
		return errors.New("unimplemented")
	case reflect.Struct:
		return unmarshalStruct(v, rv)
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return fmt.Errorf("cannot unmarshal into type %s", rv.Type())
	default:
		if len(v.Head) != 1 || len(v.List) != 0 {
			return errors.New("scalar requires Value with only one Head element")
		}
		return unmarshalScalar(v.Head[0], rv)
	}
}

func unmarshalSliceOrArray(vs []Value, rv reflect.Value) error {
	if len(vs) > rv.Len() {
		if rv.Kind() == reflect.Array {
			return fmt.Errorf("array too short: need %d, have %d", len(vs), rv.Len())
		}
		// Extend slice.
		for range len(vs) - rv.Len() {
			rv.Set(reflect.Append(rv, reflect.Zero(rv.Type().Elem())))
		}
	}
	for i, v := range vs {
		if err := unmarshalReflectValue(v, rv.Index(i)); err != nil {
			return err
		}
	}
	return nil
}

func unmarshalMap(v Value, rv reflect.Value) error {
	// Expect a value with an empty head and a list of values v where
	// each v has at least one head element, the key.
	if t := rv.Type().Key(); t.Kind() != reflect.String {
		return fmt.Errorf("map key underlying type must be string; have %s", t)
	}
	if len(v.Head) > 0 {
		return fmt.Errorf("Value for map needs empty head; have %q", v.Head)
	}
	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}
	for _, v := range v.List {
		if len(v.Head) == 0 {
			return errors.New("Value for map item needs at least one head element; have none")
		}
		elem := reflect.New(rv.Type().Elem())
		if err := unmarshalReflectValue(headRest(v), elem); err != nil {
			return err
		}
		rv.SetMapIndex(reflect.ValueOf(v.Head[0]), elem.Elem())
	}
	return nil
}

func unmarshalStruct(v Value, rv reflect.Value) error {
	// Values in v.List must have a at least one head, the field name.
	valuesByKey := map[string]Value{}
	for _, lv := range v.List {
		if len(lv.Head) == 0 {
			return errors.New("field value has no head")
		}
		key := strings.ToLower(lv.Head[0])
		if _, ok := valuesByKey[key]; ok {
			return fmt.Errorf("duplicate struct field key: %q", key)
		}
		valuesByKey[key] = lv
	}
	for _, f := range reflect.VisibleFields(rv.Type()) {
		// TODO: create the nil pointers that would cause FieldByIndexErr to return an error.
		fv, err := rv.FieldByIndexErr(f.Index)
		if err != nil {
			return err
		}
		name := f.Name
		if tag := f.Tag.Get("gdl"); tag != "" {
			name = tag
		}
		// An integer name is an index into Head.
		if i, err := strconv.Atoi(name); err == nil {
			if i <= 0 || i >= len(v.Head) {
				return fmt.Errorf("field %s: index %d is out of range of value head", f.Name, i)
			}
			if err := unmarshalScalar(v.Head[i], fv); err != nil {
				return err
			}
		} else {
			// Non-integer name: find list value case-insensitively.
			// If we can't find it, skip this field.
			if lv, ok := valuesByKey[strings.ToLower(name)]; ok {
				if err := unmarshalReflectValue(headRest(lv), fv); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func unmarshalScalar(s string, rv reflect.Value) error {
	switch rv.Kind() {
	case reflect.String:
		rv.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		rv.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		rv.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		rv.SetFloat(f)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		rv.SetBool(b)
	default:
		return fmt.Errorf("cannot unmarshal into scalar type %s", rv.Type())
	}
	return nil
}

// headRest returns a Value that is v with the first Head removed.
// It panics if v has no head.
func headRest(v Value) Value {
	v.Head = v.Head[1:]
	return v
}
