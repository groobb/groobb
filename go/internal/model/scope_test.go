package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestScopes verifies that the set holds the whole scope vocabulary and that a
// caller editing what it receives cannot change what the next caller sees. The
// set is what ScopeCommunityAdmin expands to, so either an omitted scope or
// shared mutable state could make an administrator lose an operation.
//
// [Ja] TestScopes は、集合がスコープの語彙全体を保持すること、および受け取ったものを
// 書き換える呼び出し側が、次の呼び出し側の見るものを変えられないことを検証します。
// この集合は ScopeCommunityAdmin の展開先であるため、スコープの欠落や共有された可変状態は
// 管理者が操作を失う原因になります。
func TestScopes(t *testing.T) {
	t.Parallel()

	want := []model.Scope{
		model.ScopeCommunityAdmin,
		model.ScopeUserRead,
		model.ScopeUserRoleWrite,
	}
	if got := model.Scopes(); !slices.Equal(got, want) {
		t.Errorf("Scopes() = %v, want %v", got, want)
	}

	model.Scopes()[0] = model.Scope("edited")
	if got := model.Scopes(); !slices.Equal(got, want) {
		t.Errorf("Scopes() after an edit = %v, want %v", got, want)
	}
}
