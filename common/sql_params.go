//go:build !taos_quote_params
// +build !taos_quote_params

package common

import "strings"

func interpolateParamBytes(b *strings.Builder, v []byte) {
	b.Write(v)
}

func interpolateParamString(b *strings.Builder, v string) {
	b.WriteString(v)
}
