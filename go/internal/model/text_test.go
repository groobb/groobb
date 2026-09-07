package model_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

func TestNormalizeLineBreaks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "unchanged whitespace", input: "\n  本文\t\n\n", want: "\n  本文\t\n\n"},
		{name: "CRLF", input: "\r\n本文\r\n", want: "\n本文\n"},
		{name: "lone CR", input: "\r本文\r", want: "\n本文\n"},
		{name: "mixed", input: "\r\n\r本文\n次\r\n", want: "\n\n本文\n次\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := model.NormalizeLineBreaks(tt.input); got != tt.want {
				t.Errorf("NormalizeLineBreaks(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
