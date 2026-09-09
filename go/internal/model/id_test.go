package model_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestUserID_String verifies that a UserID stringifies to the decimal form of
// the int64 it wraps.
//
// [Ja] TestUserID_String は UserID がラップする int64 の 10 進表記で文字列化される
// ことを検証します。
func TestUserID_String(t *testing.T) {
	t.Parallel()

	id := model.UserID(42)

	if got, want := id.String(), "42"; got != want {
		t.Errorf("UserID.String() = %q, want %q", got, want)
	}
}

// TestUserTwoFactorAuthID_String verifies that a UserTwoFactorAuthID stringifies
// to the decimal form of the int64 it wraps.
//
// [Ja] TestUserTwoFactorAuthID_String は UserTwoFactorAuthID がラップする int64 の
// 10 進表記で文字列化されることを検証します。
func TestUserTwoFactorAuthID_String(t *testing.T) {
	t.Parallel()

	id := model.UserTwoFactorAuthID(7)

	if got, want := id.String(), "7"; got != want {
		t.Errorf("UserTwoFactorAuthID.String() = %q, want %q", got, want)
	}
}

// TestParseUserID verifies that ParseUserID accepts the decimal form of a
// user's id and refuses everything that names no user, so that a route deciding
// whether an address can name one at all does it without a query.
//
// A spelling strconv reads but String would not write — a leading zero or a
// plus sign — parses, and the id it yields stringifies to the canonical form.
// That is what lets a caller answering under one address tell the two apart by
// comparing them.
//
// [Ja] TestParseUserID は、ParseUserID が利用者の id の 10 進表記を受け付け、
// どの利用者も名指さないものをすべて拒否することを検証します。アドレスがそもそも
// 利用者を名指しうるかを決めるルートが、クエリを発行せずにそれを行えるようにするため
// です。
//
// strconv が読み取るが String は書かない綴り (先頭のゼロやプラス記号) は解析でき、
// 得られる id は正規の形へ文字列化されます。1 つのアドレスで応答する呼び出し側が、
// 両者を突き合わせて見分けられるのはそのためです。
func TestParseUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want model.UserID
		ok   bool
	}{
		{name: "decimal id", raw: "12", want: model.UserID(12), ok: true},
		{name: "leading zero", raw: "012", want: model.UserID(12), ok: true},
		{name: "plus sign", raw: "+12", want: model.UserID(12), ok: true},
		{name: "zero", raw: "0", ok: false},
		{name: "negative", raw: "-12", ok: false},
		{name: "empty", raw: "", ok: false},
		{name: "not a number", raw: "twelve", ok: false},
		{name: "trailing text", raw: "12users", ok: false},
		{name: "beyond int64", raw: "9223372036854775808", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParseUserID(tt.raw)

			if ok != tt.ok {
				t.Fatalf("ParseUserID(%q) ok = %v, want %v", tt.raw, ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got != tt.want {
				t.Errorf("ParseUserID(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestParseThreadID verifies that ParseThreadID accepts the decimal form of a
// thread's id and refuses everything that names no thread, so that a route
// deciding whether an address can name one at all does it without a query.
//
// A spelling strconv reads but String would not write — a leading zero or a
// plus sign — parses, and the id it yields stringifies to the canonical form.
// That is what lets a caller answering under one address tell the two apart by
// comparing them.
//
// [Ja] TestParseThreadID は、ParseThreadID がスレッドの id の 10 進表記を受け付け、
// どのスレッドも名指さないものをすべて拒否することを検証します。アドレスがそもそも
// スレッドを名指しうるかを決めるルートが、クエリを発行せずにそれを行えるようにするため
// です。
//
// strconv が読み取るが String は書かない綴り (先頭のゼロやプラス記号) は解析でき、
// 得られる id は正規の形へ文字列化されます。1 つのアドレスで応答する呼び出し側が、
// 両者を突き合わせて見分けられるのはそのためです。
func TestParseThreadID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want model.ThreadID
		ok   bool
	}{
		{name: "decimal id", raw: "12", want: model.ThreadID(12), ok: true},
		{name: "leading zero", raw: "012", want: model.ThreadID(12), ok: true},
		{name: "plus sign", raw: "+12", want: model.ThreadID(12), ok: true},
		{name: "zero", raw: "0", ok: false},
		{name: "negative", raw: "-12", ok: false},
		{name: "empty", raw: "", ok: false},
		{name: "not a number", raw: "twelve", ok: false},
		{name: "trailing text", raw: "12posts", ok: false},
		{name: "beyond int64", raw: "9223372036854775808", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := model.ParseThreadID(tt.raw)

			if ok != tt.ok {
				t.Fatalf("ParseThreadID(%q) ok = %v, want %v", tt.raw, ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if got != tt.want {
				t.Errorf("ParseThreadID(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
