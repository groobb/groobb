package model_test

import (
	"slices"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestModerationActionsは、集合が履歴の記録するすべての操作を保持すること、および
// 受け取ったものを書き換える呼び出し側が、次の呼び出し側の見るものを変えられないことを
// 検証します。この集合はリポジトリが挿入の前にactionを突き合わせる相手であるため、これを
// 書き換えられる呼び出し側は、CHECKが守っていない列へ集合の外の値を通すこともできて
// しまいます。
func TestModerationActions(t *testing.T) {
	t.Parallel()

	want := []model.ModerationAction{
		model.ModerationActionThreadLock,
		model.ModerationActionThreadUnlock,
		model.ModerationActionThreadUnpublish,
		model.ModerationActionPostUnpublish,
		model.ModerationActionUserSuspend,
		model.ModerationActionUserUnsuspend,
	}
	if got := model.ModerationActions(); !slices.Equal(got, want) {
		t.Errorf("ModerationActions() = %v、期待値 = %v", got, want)
	}

	model.ModerationActions()[0] = model.ModerationAction("edited")
	if got := model.ModerationActions(); !slices.Equal(got, want) {
		t.Errorf("書き換えた後のModerationActions() = %v、期待値 = %v", got, want)
	}
}

// TestModerationAction_IsValidは、記録される操作がいずれも通ること、そして集合の外の
// 値が拒否されることを検証します。列はCHECKで値を列挙しないため、この答えが履歴を読める
// 状態に保つもののすべてです。何とも名指せないactionは、何かが起きたことだけを述べて何が
// 起きたかを述べない行を残します。
func TestModerationAction_IsValid(t *testing.T) {
	t.Parallel()

	for _, action := range model.ModerationActions() {
		if !action.IsValid() {
			t.Errorf("ModerationAction(%q).IsValid() = false、期待値 = true", action)
		}
	}

	invalid := []model.ModerationAction{
		"",
		"thread_delete",
		"Thread_Lock",
		"user_suspend ",
	}
	for _, action := range invalid {
		if action.IsValid() {
			t.Errorf("ModerationAction(%q).IsValid() = true、期待値 = false", action)
		}
	}
}
