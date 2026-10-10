//go:build taos_quote_params
// +build taos_quote_params

package common

import "strings"

func interpolateParamBytes(b *strings.Builder, v []byte) {
	b.WriteByte('\'')
	for _, c := range v {
		writeEscapedChar(b, c)
	}
	b.WriteByte('\'')
}

func interpolateParamString(b *strings.Builder, v string) {
	b.WriteByte('\'')
	for i := 0; i < len(v); i++ {
		writeEscapedChar(b, v[i])
	}
	b.WriteByte('\'')
}

// writeEscapedChar writes one byte of a single-quoted string literal body.
// Backslash and single quote are escaped because they delimit the literal.
// Double quote, newline, carriage return and tab are escaped via their
// documented TDengine sequences as a defensive measure. Every other byte,
// including NUL (0x00) and 0x1A, is written unchanged.
func writeEscapedChar(b *strings.Builder, c byte) {
	switch c {
	case '\\':
		b.WriteString(`\\`)
	case '\'':
		b.WriteString(`\'`)
	case '"':
		b.WriteString(`\"`)
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	default:
		b.WriteByte(c)
	}
}
