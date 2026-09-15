package model_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestUserID_StringはUserIDがラップするint64の10進表記で文字列化される
// ことを検証します。
func TestUserID_String(t *testing.T) {
	t.Parallel()

	id := model.UserID(42)

	if got, want := id.String(), "42"; got != want {
		t.Errorf("UserID.String() = %q、期待値 = %q", got, want)
	}
}

// TestUserTwoFactorAuthID_StringはUserTwoFactorAuthIDがラップするint64の
// 10進表記で文字列化されることを検証します。
func TestUserTwoFactorAuthID_String(t *testing.T) {
	t.Parallel()

	id := model.UserTwoFactorAuthID(7)

	if got, want := id.String(), "7"; got != want {
		t.Errorf("UserTwoFactorAuthID.String() = %q、期待値 = %q", got, want)
	}
}

// TestParseUserIDは、ParseUserIDが利用者のidの10進表記を受け付け、
// どの利用者も名指さないものをすべて拒否することを検証します。アドレスがそもそも
// 利用者を名指しうるかを決めるルートが、クエリを発行せずにそれを行えるようにするため
// です。
//
// strconvが読み取るがStringは書かない綴り (先頭のゼロやプラス記号) は解析でき、
// 得られるidは正規の形へ文字列化されます。1つのアドレスで応答する呼び出し側が、
// 両者を突き合わせて見分けられるのはそのためです。
func TestParseUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want model.UserID
		ok   bool
	}{
		{name: "10進数のID", raw: "12", want: model.UserID(12), ok: true},
		{name: "先頭にゼロが付く", raw: "012", want: model.UserID(12), ok: true},
		{name: "プラス記号が付く", raw: "+12", want: model.UserID(12), ok: true},
		{name: "ゼロ", raw: "0", ok: false},
		{name: "負の数", raw: "-12", ok: false},
		{name: "空文字列", raw: "", ok: false},
		{name: "数値でない", raw: "twelve", ok: false},
		{name: "数字の後ろに文字が続く", raw: "12users", ok: false},
		{name: "int64の範囲を超える", raw: "9223372036854775808", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParseUserID(tt.raw)

			if ok != tt.ok {
				t.Fatalf("ParseUserID(%q)のok = %v、期待値 = %v", tt.raw, ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got != tt.want {
				t.Errorf("ParseUserID(%q) = %v、期待値 = %v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestParseThreadIDは、ParseThreadIDがスレッドのidの10進表記を受け付け、
// どのスレッドも名指さないものをすべて拒否することを検証します。アドレスがそもそも
// スレッドを名指しうるかを決めるルートが、クエリを発行せずにそれを行えるようにするため
// です。
//
// strconvが読み取るがStringは書かない綴り (先頭のゼロやプラス記号) は解析でき、
// 得られるidは正規の形へ文字列化されます。1つのアドレスで応答する呼び出し側が、
// 両者を突き合わせて見分けられるのはそのためです。
func TestParseThreadID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want model.ThreadID
		ok   bool
	}{
		{name: "10進数のID", raw: "12", want: model.ThreadID(12), ok: true},
		{name: "先頭にゼロが付く", raw: "012", want: model.ThreadID(12), ok: true},
		{name: "プラス記号が付く", raw: "+12", want: model.ThreadID(12), ok: true},
		{name: "ゼロ", raw: "0", ok: false},
		{name: "負の数", raw: "-12", ok: false},
		{name: "空文字列", raw: "", ok: false},
		{name: "数値でない", raw: "twelve", ok: false},
		{name: "数字の後ろに文字が続く", raw: "12posts", ok: false},
		{name: "int64の範囲を超える", raw: "9223372036854775808", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParseThreadID(tt.raw)

			if ok != tt.ok {
				t.Fatalf("ParseThreadID(%q)のok = %v、期待値 = %v", tt.raw, ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got != tt.want {
				t.Errorf("ParseThreadID(%q) = %v、期待値 = %v", tt.raw, got, tt.want)
			}
		})
	}
}
