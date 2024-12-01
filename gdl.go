// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: support unmarshaling into any.

// TODO: support a struct tag like "2-" to mean from arg 2 to the end.

// TODO: rules on reuse:
// If you re-use a data structure for unmarshaling, beware:
// - You may have a value set that does not appear in the second input; it will still be there.
// - That goes for struct fields, array/slice elements, map items.
// (Consider a "clear" mode that zeros these things and sets slice lengths; but
// maybe it's not worth it.)
// (It certainly doesn't make sense to do this for slices/arrays but not for struct fields; that
// would just be inconsistent. All or nothing.)

// gdl implements a decription language inspired by the file format of go.mod
// and similar files.
//
// # Lexical structure
//
// A gdl string is a sequence of words, separators and delimiters.
// The delimiters are parentheses and braces.
// The separators are newline and semicolon.
// If the last non-whitespace character on a line is a backslash, the line continues
// onto the next line.
//
// A word is a sequence of non-space characters ending in a delimiter or separator.
// Words can be double-quoted or backquoted as in Go, with the same syntax.
// Comments begin with // and extend to the end of the line.
// Backslashes are ignored inside a comment.
//
// # Values
//
// A [Value] has two parts: a head and a list.
// The head is a sequence of words. For example,
//
//	name Al Jones
//
// is a Value whose head has three words and whose list is empty.
// A list is a sequence of Values, delimited by braces.
// This is a list with three Values, each of which has a single head word
// and no list:
//
//	{ one; two; three }
//
// Here is a Value with both a head and a list:
//
//	command create {
//	    description "create a shape"
//	    args size position
//	}
//
// The head of this Value has the two words "command" and "create".
// The list consists of two Values, one on each line.
//
// A sequence of values surrounded by parentheses repeats those values using the head as a prefix.
// For example,
//
//	  require (
//	      example.com/a v1.2.3
//	      example.com/b v0.2.5
//	  )
//
//	is equivalent to the two values
//
//	  require example.com/a v1.2.3
//	  require example.com/b v0.2.5
//
// Use the [Values] and [FileValues] functions to parse a sequence of values from a string or file, respectively.
//
// # Unmarshaling
//
// A Value can be unmarshaled into a struct or other Go type. A Value with no list
// and a one-element head can be unmarshaled into a Go scalar like a string or int.
// For example, the value
//
//	1
//
// can be unmarshaled into a string, int or float.
//
// A value with multiple head words, or with a list, can be unmarshaled into a slice or array.
// More complicated values can be unmarshaled into a map or struct.
// See [UnmarshalValue] for details.
//
// A sequence of Values, such as an entire file, can be unmarshaled into a struct with [UnmarshalValues].
package gdl

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Value struct {
	Head []string
	List []Value
	File string
	Line int
}

func (v Value) Pos() string {
	if v.File == "" && v.Line == 0 {
		return "?"
	}
	if v.Line == 0 {
		return v.File
	}
	if v.File == "" {
		return fmt.Sprintf("?:%d", v.Line)
	}
	return fmt.Sprintf("%s:%d", v.File, v.Line)
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

// Parse returns a slice of the Values in s.
func Parse(s string) ([]Value, error) {
	iter, errf := Values(s)
	vals := slices.Collect(iter)
	if err := errf(); err != nil {
		return nil, err
	}
	return vals, nil
}

// parse parses s into a sequence of Values. The filename is only for display in errors.
func parse(s, filename string) (iter.Seq[Value], func() error) {
	var err error
	iter := func(yield func(Value) bool) {
		lex := newLexer(s, filename)
		for {
			tok := skipNewlines(lex)
			switch tok.kind {
			case tokEOF:
				return
			case ')':
				err = fmt.Errorf("%s:%d: unexpected close paren", lex.filename, lex.lineno)
				return
			default:
				vals, e := parseValues(tok, lex)
				if e != nil {
					e = fmt.Errorf("%s:%d: %w", lex.filename, lex.lineno, e)
					return
				}
				for _, v := range vals {
					if !yield(v) {
						return
					}
				}
			}
		}
	}
	return iter, func() error { return err }
}

func newValue(head []string, list []Value, lex *lexer) Value {
	return Value{
		Head: head,
		List: list,
		File: lex.filename,
		Line: lex.lineno,
	}
}

// Called at line start. Ends at the next line start or EOF.
// Only called when there is a value.
func parseValues(tok token, lex *lexer) ([]Value, error) {
	var head []string
	for {
		switch tok.kind {
		case tokEOF:
			// Accept a value that isn't followed by a newline.
			if len(head) > 0 {
				return []Value{newValue(head, nil, lex)}, nil
			}
			return nil, io.ErrUnexpectedEOF

		case '\n':
			if len(head) > 0 {
				return []Value{newValue(head, nil, lex)}, nil
			}
			return nil, errors.New("unexpected newline")
			// // blank line
			// continue

		case tokWord:
			head = append(head, tok.val)

		case tokString:
			unq, err := strconv.Unquote(tok.val)
			if err != nil {
				return nil, err
			}
			head = append(head, unq)

		case '(':
			list, err := parseList(lex, ')')
			if err != nil {
				return nil, err
			}
			var vals []Value
			for _, lv := range list {
				vals = append(vals, newValue(slices.Concat(head, lv.Head), lv.List, lex))
			}
			return vals, nil

		case ')', '}':
			lex.unget(tok)
			if len(head) == 0 {
				panic("bad close delimiter")
			}
			// We're here after getting b in something like
			//    (a; b)
			// The close delim is part of the enclosing list.
			return []Value{newValue(head, nil, lex)}, nil

		case '{':
			list, err := parseList(lex, '}')
			if err != nil {
				return nil, err
			}
			return []Value{newValue(head, list, lex)}, nil

		case tokErr:
			return nil, tok.err

		default:
			panic("bad token kind")
		}
		tok = lex.next()
	}
}

// Called just after open delimiter. Ends just after close delimiter.
func parseList(lex *lexer, close rune) ([]Value, error) {
	var vs []Value
	for {
		tok := skipNewlines(lex)
		switch tok.kind {
		case tokEOF:
			return nil, io.ErrUnexpectedEOF
		case close:
			k := lex.peek()
			switch k {
			case tokErr:
				return nil, lex.next().err
			case close, '\n', tokEOF:
				return vs, nil
			default:
				return nil, errors.New("close delimiter must be followed by newline, EOF or another close delimiter")
			}
		case ')', '}':
			return nil, errors.New("mismatched close delimiter")
		}
		vs1, err := parseValues(tok, lex)
		if err != nil {
			return nil, err
		}
		vs = append(vs, vs1...)
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

// UnmarshalValues takes a sequence of Values and unmarshals them into p.
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
			return fmt.Errorf("%s: value has no head", val.Pos())
		}
		if len(val.Head[0]) == 0 {
			return fmt.Errorf("%s: value has empty head", val.Pos())
		}
		key := val.Head[0]
		sf, ok := matchField(key, sfs)
		if !ok {
			return fmt.Errorf("%s: no field in struct matches %q", val.Pos(), key)
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
				return fmt.Errorf("%s: key %q occurs more than once", val.Pos(), key)
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

// UnmarshalValue unmarshals a [Value] into a Go value.
// The second argument must be a pointer.
//
// If p points to a string, int, uint, float or bool, v must have no list and one head word.
// It is converted using the [strconv] package.
//
// If p points to a slice or array, v must have no list, or no head.
// The non-empty part is assigned to the slice or array element by element.
// Slices are appended to as needed.
//
// If p points to a map, the map key type must be a string, and v cannot have a head.
// Each Value in v's list must have at least one head word.
// That word is the map key, and the rest of the Value is unmarshaled into a new Go value
// of the map's value type.
// For example, TODO.
//
// TODO: struct
func UnmarshalValue(v Value, p any) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%s: unmarshaling into %T: %w", v.Pos(), p, err)
		}
	}()

	return unmarshalReflectValue(v, reflect.ValueOf(p))
}

func unmarshalReflectValue(v Value, rv reflect.Value) error {

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
				vs = append(vs, Value{Head: []string{h}, File: v.File, Line: v.Line})
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
			return errors.New("scalar requires Value with only one head element")
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
	saw := map[any]bool{}
	sawNonStarString := false
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
		// The name "*" means use the entire list, without field prefixes.
		// The field must be a slice or array.
		if name == "*" {
			if saw["*"] || sawNonStarString {
				return fmt.Errorf("field %s: saw '*' and field name or other '*' in same struct", f.Name)
			}
			saw["*"] = true
			v2 := v
			v2.Head = nil
			if err := unmarshalReflectValue(v2, fv); err != nil {
				return err
			}
		} else if i, err := strconv.Atoi(name); err == nil {
			// An integer name is an index into Head.
			if i <= 0 || i >= len(v.Head) {
				return fmt.Errorf("field %s: index %d is out of range of value head", f.Name, i)
			}
			if saw[i] {
				return fmt.Errorf("field %s: duplicate index %d", f.Name, i)
			}
			saw[i] = true
			if err := unmarshalScalar(v.Head[i], fv); err != nil {
				return err
			}
		} else {
			// Non-integer name: find list value case-insensitively.
			// If we can't find it, skip this field.
			if saw[name] || saw["*"] {
				return fmt.Errorf("field %s: duplicate name %q or saw '*'", f.Name, name)
			}
			saw[name] = true
			sawNonStarString = true
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
