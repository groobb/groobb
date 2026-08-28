package model_test

import (
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
)

// TestIsValidSlug verifies the rule the path helpers rely on: a slug is lowercase
// ASCII letters, digits, hyphens and underscores, at most SlugMaxLength of them,
// and carries nothing that would change where /b/{slug} points.
//
// [Ja] TestIsValidSlug は、パスヘルパーが前提にしている規則を検証します。slug は ASCII の
// 英小文字・数字・ハイフン・アンダースコアで、SlugMaxLength 文字以内であり、/b/{slug} の
// 指す先を変えてしまうものを含みません。
func TestIsValidSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		slug string
		want bool
	}{
		{name: "英小文字", slug: "chat", want: true},
		{name: "ハイフン区切り", slug: "game-talk", want: true},
		{name: "アンダースコア区切り", slug: "game_talk", want: true},
		{name: "数字と英小文字", slug: "board2", want: true},
		{name: "最大長ちょうど", slug: strings.Repeat("a", model.SlugMaxLength), want: true},
		{name: "空文字", slug: "", want: false},
		{name: "最大長超過", slug: strings.Repeat("a", model.SlugMaxLength+1), want: false},
		{name: "スラッシュを含む", slug: "games/rpg", want: false},
		{name: "クエリの開始文字を含む", slug: "games?x=1", want: false},
		{name: "フラグメントの開始文字を含む", slug: "games#top", want: false},
		{name: "空白を含む", slug: "game talk", want: false},
		{name: "大文字を含む", slug: "Board2", want: false},
		{name: "非 ASCII を含む", slug: "ゲーム", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := model.IsValidSlug(tt.slug); got != tt.want {
				t.Errorf("IsValidSlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}
