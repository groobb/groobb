package layouts

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// renderDefaultは与えたchildren本文でDefaultを描画し、HTML文字列を返す。
func renderDefault(t *testing.T, lang, title, bodyHTML string) string {
	t.Helper()

	ctx := templ.WithChildren(context.Background(), templ.Raw(bodyHTML))

	var buf bytes.Buffer
	if err := Default(lang, title).Render(ctx, &buf); err != nil {
		t.Fatalf("Default(%q, %q).Render()のエラー = %v", lang, title, err)
	}
	return buf.String()
}

func TestDefault_Japanese(t *testing.T) {
	t.Parallel()

	html := renderDefault(t, "ja", "確認用コード", "<p>BODY_MARKER</p>")

	// templは描画時にdoctypeを小文字化する。
	if !strings.Contains(html, "<!doctype html>") {
		t.Errorf("HTMLにdoctypeが含まれていない: %s", html)
	}
	if !strings.Contains(html, `lang="ja"`) {
		t.Error("HTMLにlang=jaが含まれていない")
	}
	if !strings.Contains(html, "<title>確認用コード</title>") {
		t.Error("HTMLにタイトルが含まれていない")
	}
	// children本文がレイアウト内に描画されている。
	if !strings.Contains(html, "BODY_MARKER") {
		t.Error("HTMLにchildrenの本文が含まれていない")
	}
	// 共有フッターにGroobbの署名が入る。
	if !strings.Contains(html, "Groobb") {
		t.Error("HTMLにフッターの署名が含まれていない")
	}
}

func TestDefault_English(t *testing.T) {
	t.Parallel()

	html := renderDefault(t, "en", "Confirmation code", "<p>BODY_MARKER</p>")

	if !strings.Contains(html, `lang="en"`) {
		t.Error("HTMLにlang=enが含まれていない")
	}
	if !strings.Contains(html, "<title>Confirmation code</title>") {
		t.Error("HTMLにタイトルが含まれていない")
	}
	if !strings.Contains(html, "BODY_MARKER") {
		t.Error("HTMLにchildrenの本文が含まれていない")
	}
}

func TestDefault_EscapesTitle(t *testing.T) {
	t.Parallel()

	// HTMLメタ文字を含むtitleは解釈されずエスケープされること。ユーザー由来の
	// テキストから組まれた件名がマークアップを注入できないようにするため。
	html := renderDefault(t, "ja", "<script>", "<p>body</p>")

	if strings.Contains(html, "<title><script></title>") {
		t.Error("タイトルがエスケープされていない")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("タイトルがHTMLエスケープされていない")
	}
}
