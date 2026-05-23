package tools

import (
	"fmt"
	"strings"
)

// writeIntLine writes "label: value\n" when value is non-zero.
func writeIntLine(buf *strings.Builder, label string, value int) {
	if value > 0 {
		fmt.Fprintf(buf, "%s: %d\n", label, value)
	}
}

// writeStringLine writes "label: value\n" when value is non-empty.
func writeStringLine(buf *strings.Builder, label, value string) {
	if value != "" {
		fmt.Fprintf(buf, "%s: %s\n", label, value)
	}
}
