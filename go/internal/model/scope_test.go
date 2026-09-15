package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestScopesは、集合がスコープの語彙全体を保持すること、および受け取ったものを
// 書き換える呼び出し側が、次の呼び出し側の見るものを変えられないことを検証します。
// この集合はScopeCommunityAdminの展開先であるため、スコープの欠落や共有された可変状態は
// 管理者が操作を失う原因になります。
func TestScopes(t *testing.T) {
	t.Parallel()

	want := []model.Scope{
		model.ScopeCommunityAdmin,
		model.ScopeUserRead,
		model.ScopeUserRoleWrite,
		model.ScopeThreadLockWrite,
		model.ScopeThreadUnpublicationWrite,
		model.ScopePostUnpublicationWrite,
		model.ScopeUserSuspensionWrite,
		model.ScopeModerationLogRead,
	}
	if got := model.Scopes(); !slices.Equal(got, want) {
		t.Errorf("Scopes() = %v、期待値 = %v", got, want)
	}

	model.Scopes()[0] = model.Scope("edited")
	if got := model.Scopes(); !slices.Equal(got, want) {
		t.Errorf("書き換えた後のScopes() = %v、期待値 = %v", got, want)
	}
}
