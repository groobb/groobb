package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestFlashはFlashがフラッシュメッセージをBasecoatのtoastとして描画し、
// メッセージ本文・種別に対応するcategory・(ローカライズされた) 閉じるボタンを持ち、
// エラーのときrole="alert"、それ以外はrole="status" になることを検証します。
// また、メッセージがnilのときは何も描画しないことも検証します。
func TestFlash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		flash           *session.FlashMessage
		wantContains    []string
		wantNotContains []string
		wantEmpty       bool
	}{
		{
			name:  "成功のメッセージは成功のトーストを描画する",
			flash: &session.FlashMessage{Type: session.FlashSuccess, Message: "ログアウトしました"},
			wantContains: []string{
				`id="toaster"`,
				`class="toast"`,
				`role="status"`,
				`data-category="success"`,
				"ログアウトしました",
				"data-toast-action",
				"閉じる",
			},
			wantNotContains: []string{
				`data-category="error"`,
				`data-category="warning"`,
				`data-category="info"`,
			},
		},
		{
			name:  "エラーのメッセージはassertiveなエラーのトーストを描画する",
			flash: &session.FlashMessage{Type: session.FlashError, Message: "エラーが発生しました"},
			wantContains: []string{
				`role="alert"`,
				`data-category="error"`,
				"エラーが発生しました",
			},
			wantNotContains: []string{
				`data-category="success"`,
				`data-category="warning"`,
				`data-category="info"`,
			},
		},
		{
			name:  "警告のメッセージは警告のトーストを描画する",
			flash: &session.FlashMessage{Type: session.FlashWarning, Message: "注意メッセージ"},
			wantContains: []string{
				`role="status"`,
				`data-category="warning"`,
				"注意メッセージ",
			},
			wantNotContains: []string{
				`data-category="success"`,
				`data-category="error"`,
				`data-category="info"`,
			},
		},
		{
			name:  "お知らせのメッセージはお知らせのトーストを描画する",
			flash: &session.FlashMessage{Type: session.FlashInfo, Message: "お知らせ"},
			wantContains: []string{
				`role="status"`,
				`data-category="info"`,
				"お知らせ",
			},
			wantNotContains: []string{
				`data-category="success"`,
				`data-category="error"`,
				`data-category="warning"`,
			},
		},
		{
			name:      "メッセージがnilなら何も描画しない",
			flash:     nil,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), "ja")

			var buf strings.Builder
			if err := components.Flash(tt.flash).Render(ctx, &buf); err != nil {
				t.Fatalf("描画に失敗: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("出力 = %q、空を期待", got)
				}
				return
			}

			// 属性コンテキストでのelse-if連鎖はtoastタグにリテラルの " else" を
			// 混入させる形で壊れるため、その回帰を全種別で防ぐ。
			if strings.Contains(got, " else") {
				t.Errorf("出力に余計な %q 属性が含まれている\n出力: %s", " else", got)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("出力に %q が含まれている\n出力: %s", notWant, got)
				}
			}
		})
	}
}
