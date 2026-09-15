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
		{name: "空文字列", input: "", want: ""},
		{name: "空白はそのまま", input: "\n  本文\t\n\n", want: "\n  本文\t\n\n"},
		{name: "CRLFの改行", input: "\r\n本文\r\n", want: "\n本文\n"},
		{name: "単独のCR", input: "\r本文\r", want: "\n本文\n"},
		{name: "改行の混在", input: "\r\n\r本文\n次\r\n", want: "\n\n本文\n次\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := model.NormalizeLineBreaks(tt.input); got != tt.want {
				t.Errorf("NormalizeLineBreaks(%q) = %q、期待値 = %q", tt.input, got, tt.want)
			}
		})
	}
}
