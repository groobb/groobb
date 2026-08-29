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

// renderThreadLanguageBadge renders the badge for language with the page drawn
// in locale, and returns the markup.
//
// [Ja] renderThreadLanguageBadge は、ページを locale で描いた状態で language の
// バッジを描画し、そのマークアップを返します。
func renderThreadLanguageBadge(t *testing.T, locale model.Locale, language model.ThreadLanguage) string {
	t.Helper()

	ctx := i18n.SetLocale(context.Background(), locale)

	var buf bytes.Buffer
	if err := components.ThreadLanguageBadge(viewmodel.NewThreadLanguage(language)).Render(ctx, &buf); err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	return buf.String()
}

// TestThreadLanguageBadge verifies that a thread written in a display language
// is badged with that language's own name, declared as that language, and
// preceded by a visually hidden label in the language of the page. The name is
// the same whichever language the page is drawn in, since it is what its own
// speakers recognise.
//
// [Ja] TestThreadLanguageBadge は、表示言語で書かれたスレッドが、その言語自身の名前で
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
			name:      "Japanese thread on a Japanese page",
			locale:    model.LocaleJa,
			language:  model.LocaleJa.ThreadLanguage(),
			wantLabel: "主言語:",
			wantName:  "日本語",
			wantTag:   "ja",
		},
		{
			name:      "English thread on a Japanese page",
			locale:    model.LocaleJa,
			language:  model.LocaleEn.ThreadLanguage(),
			wantLabel: "主言語:",
			wantName:  "English",
			wantTag:   "en",
		},
		{
			name:      "Japanese thread on an English page",
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
				t.Errorf("バッジの要素 = %s, want span.badge with the outline variant", badge)
			}

			// The label is inside the badge but outside the element carrying the
			// lang, since it is written in the language of the page rather than in
			// the thread's. Taking the name out of the declared element is what
			// says the two are not covered by one declaration.
			//
			// [Ja] ラベルはバッジの内側かつ lang を持つ要素の外側にある。ラベルが書かれて
			// いるのはスレッドの言語ではなくページの言語だからである。名前を宣言された
			// 要素から取り出すことが、2 つが 1 つの宣言に覆われていないことを述べる。
			if want := `<span class="sr-only">` + tt.wantLabel + `</span>`; !strings.Contains(markup, want) {
				t.Errorf("バッジ = %s, want the visually hidden label %q", markup, want)
			}
			if want := `<span lang="` + tt.wantTag + `">` + tt.wantName + `</span>`; !strings.Contains(markup, want) {
				t.Errorf("バッジ = %s, want the language's own name declared as %q", markup, tt.wantTag)
			}
		})
	}
}

// TestThreadLanguageBadge_Other verifies that a thread whose language resolves
// to no display language is badged with the translated word for it and declares
// no language. There is no tag to declare, and an invented one would have a
// screen reader pronounce the badge by the rules of a language the thread is not
// written in.
//
// [Ja] TestThreadLanguageBadge_Other は、どの表示言語にも解決しない言語のスレッドが、
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
		{name: "Japanese", locale: model.LocaleJa, want: "その他"},
		{name: "English", locale: model.LocaleEn, want: "Other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			markup := renderThreadLanguageBadge(t, tt.locale, model.ThreadLanguageOther)

			if !strings.Contains(markup, tt.want) {
				t.Errorf("バッジ = %s, want the translated word %q", markup, tt.want)
			}
			if strings.Contains(markup, "lang=") {
				t.Errorf("バッジ = %s, want no lang declaration", markup)
			}
		})
	}
}
