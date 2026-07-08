package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestFlash verifies that Flash renders a Basecoat toast for a flash message,
// carrying the message text, the category matching its type, the dismiss button
// (localized), and role="alert" for errors versus role="status" otherwise, and
// that it renders nothing at all when the message is nil.
//
// [Ja] TestFlash は Flash がフラッシュメッセージを Basecoat の toast として描画し、
// メッセージ本文・種別に対応する category・(ローカライズされた) 閉じるボタンを持ち、
// エラーのとき role="alert"、それ以外は role="status" になることを検証します。
// また、メッセージが nil のときは何も描画しないことも検証します。
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
			name:  "success message renders a success toast",
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
			name:  "error message renders an assertive error toast",
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
			name:  "warning message renders a warning toast",
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
			name:  "info message renders an info toast",
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
			name:      "nil message renders nothing",
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
				t.Fatalf("render failed: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("expected no output, got %q", got)
				}
				return
			}

			// An else-if chain in an attribute context miscompiles into a stray
			// literal " else" leaking into the toast tag; guard against that
			// regression across every category.
			//
			// [Ja] 属性コンテキストでの else-if 連鎖は toast タグにリテラルの " else" を
			// 混入させる形で壊れるため、その回帰を全種別で防ぐ。
			if strings.Contains(got, " else") {
				t.Errorf("output contains a stray %q attribute\noutput: %s", " else", got)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q\noutput: %s", want, got)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("output should not contain %q\noutput: %s", notWant, got)
				}
			}
		})
	}
}
