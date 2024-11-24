# gdl

GDL is a declarative language inspired by the format of files like go.mod
and go.work.

It can be used for configuration or declarative specifications.

## Syntax

A file is sequence of lines.
Blank lines and lines beginning with "//" are ignored.

Each line is split into words on whitespace. Exceptions:
- Double-quoted strings, Go syntax.
- Backquoted strings, Go syntax (they may span lines).
- A final backslash continues the line.

A word is interpreted as an int, float, bool or string, according to Go syntax
(except that strings don't require quotation). Only "true" and "false" are bools;
other strings acceptable to strconv.ParseBool, like "t" and "FALSE", are not.

If the last word is "(", the other words in the line become a prefix to the
following lines, up to a line consisting only of ")".

Example:
```
foo (
  bar
  baz
)
```
is equivalent to
```
foo bar
foo baz
```

## Mapping To JSON

A JSON object `{"a": 1, "b": 2}` could be represented as
```
a 1
b 2
```

A JSON list `[1, 2, 3]` could be represented as
```
(
  1
  2
  3
)
```
