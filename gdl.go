// Copyright 2024 by Jonathan Amsterdam.
// Use of this source code is governed by a license
// that can be found in the LICENSE file.

// TODO: allow open paren to be followed by a value (but not close, that would be confusing)

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

// A word that looks like an integer is converted to an int64.
// A word that looks like a float-point number is converted to a float64.
// The words true and false are converted to bools.
// No other processing is done to a word.
package gdl

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type Value struct {
	Head []string
	List []Value
}

// ParseFile calls [Parse] on the contents of the file.
// The file is an implicit list of values.
func ParseFile(filename string) ([]Value, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(string(data), filename)
}

// Parse parses the string and returns a slice of [Value]s.
// Each element of a line slice is either a string, int64, float64 or bool.
func Parse(s string) ([]Value, error) {
	return parse(s, "<no file>")
}

// parse parses s. The filename is only for display in errors.
func parse(s, filename string) (_ []Value, err error) {
	lex := newLexer(s)
	var vals []Value
	for {
		tok := skipNewlines(lex)
		switch tok.kind {
		case tokEOF:
			return vals, nil
		case ')':
			return nil, fmt.Errorf("%s:%d: unexpected close paren", filename, lex.lineno)
		default:
			val, err := parseValue(tok, lex)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", filename, lex.lineno, err)
			}
			vals = append(vals, val)
		}
	}
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
			// Expect newline or EOF.
			if tok := lex.next(); tok.kind != '\n' && tok.kind != tokEOF {
				return nil, cmp.Or(tok.err, errors.New("close paren must be followed by newline or EOF"))
			}
			return vs, nil
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

// func UnmarshalValues(vals []Value, p any) error {
// }

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
		// Can json.Unmarshal take a straight slice, or does it need ptr to slice?
		// If head is empty, unmarshal each value into an element.
		if len(v.Head) == 0 {
			for i := 0; i < min(rv.Len(), len(v.List)); i++ {
				if err := unmarshalReflectValue(v.List[i], rv.Index(i)); err != nil {
					return err
				}
			}
		} else if len(v.List) == 0 {
			for i := 0; i < min(rv.Len(), len(v.Head)); i++ {
				if err := unmarshalScalar(v.Head[i], rv.Index(i)); err != nil {
					return err
				}
			}
		} else {
			return errors.New("cannot unmarshal into slice a value that has both a head and a list")
		}
		// Don't clear or set length unless we want to do it for everything, even struct fields.
		return nil

	case reflect.Map:
		// Single head is key?
		return errors.New("unimplemented")
	case reflect.Pointer:
		// Use reflect.New and recurse with that? Unless it's not nil.
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

func unmarshalStruct(v Value, rv reflect.Value) error {
	// Values in list must have a at least one head, the field name.
	// rt := rv.Type()
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
				// Remove the key from the head.
				lv.Head = lv.Head[1:]
				if err := unmarshalReflectValue(lv, fv); err != nil {
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
