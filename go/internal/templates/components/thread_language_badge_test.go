package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// renderThreadLanguageBadgeは、ページをlocaleで描いた状態でlanguageの
// バッジを描画し、そのマークアップを返します。
func renderThreadLanguageBadge(t *testing.T, locale model.Locale, language model.ThreadLanguage) string {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), locale)

	var buf bytes.Buffer
	if err := components.ThreadLanguageBadge(viewmodel.NewThreadLanguage(language)).Render(ctx, &buf); err != nil {
		t.Fatalf("描画に失敗: %v", err)
	}

	return buf.String()
}

// TestThreadLanguageBadgeは、表示言語で書かれたスレッドが、その言語自身の名前で
// バッジに示され、その言語として宣言され、ページの言語で書かれた視覚的に隠したラベルを
// 前に持つことを検証します。名前はページがどの言語で描かれていても同じです。それが
// その言語の話者が認識するものだからです。
func TestThreadLanguageBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    model.Locale
		language  model.ThreadLanguage
		wantLabel string
		wantName  string
		wantTag   string
	}{
		{
			name:      "日本語のページの日本語のスレッド",
			locale:    model.LocaleJa,
			language:  model.LocaleJa.ThreadLanguage(),
			wantLabel: "主言語:",
			wantName:  "日本語",
			wantTag:   "ja",
		},
		{
			name:      "日本語のページの英語のスレッド",
			locale:    model.LocaleJa,
			language:  model.LocaleEn.ThreadLanguage(),
			wantLabel: "主言語:",
			wantName:  "English",
			wantTag:   "en",
		},
		{
			name:      "英語のページの日本語のスレッド",
			locale:    model.LocaleEn,
			language:  model.LocaleJa.ThreadLanguage(),
			wantLabel: "Primary language:",
			wantName:  "日本語",
			wantTag:   "ja",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			markup := renderThreadLanguageBadge(t, tt.locale, tt.language)

			badge := testutil.OpeningTag(t, markup, `class="badge"`)
			if !strings.HasPrefix(badge, "<span ") || !strings.Contains(badge, `data-variant="outline"`) {
				t.Errorf("バッジの要素 = %s、期待値はoutlineのバリアントを持つspan.badge", badge)
			}

			// ラベルはバッジの内側かつlangを持つ要素の外側にある。ラベルが書かれて
			// いるのはスレッドの言語ではなくページの言語だからである。名前を宣言された
			// 要素から取り出すことが、2つが1つの宣言に覆われていないことを述べる。
			if want := `<span class="sr-only">` + tt.wantLabel + `</span>`; !strings.Contains(markup, want) {
				t.Errorf("バッジ = %s、視覚的に隠したラベル %q を含むことを期待", markup, want)
			}
			if want := `<span lang="` + tt.wantTag + `">` + tt.wantName + `</span>`; !strings.Contains(markup, want) {
				t.Errorf("バッジ = %s、%q と宣言された言語自身の名前を含むことを期待", markup, tt.wantTag)
			}
		})
	}
}

// TestThreadLanguageBadge_Otherは、どの表示言語にも解決しない言語のスレッドが、
// その訳語でバッジに示され、どの言語も宣言しないことを検証します。宣言するタグは無く、
// でっち上げたタグは、そのスレッドが書かれていない言語の規則でスクリーンリーダーに
// バッジを発音させることになります。
func TestThreadLanguageBadge_Other(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale model.Locale
		want   string
	}{
		{name: "日本語", locale: model.LocaleJa, want: "その他"},
		{name: "英語", locale: model.LocaleEn, want: "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			markup := renderThreadLanguageBadge(t, tt.locale, model.ThreadLanguageOther)

			if !strings.Contains(markup, tt.want) {
				t.Errorf("バッジ = %s、訳語 %q を含むことを期待", markup, tt.want)
			}
			if strings.Contains(markup, "lang=") {
				t.Errorf("バッジ = %s、langの宣言が無いことを期待", markup)
			}
		})
	}
}
