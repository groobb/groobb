package layouts

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// renderDefault renders Default with the given children body and returns the
// HTML string.
//
// [Ja] renderDefault は与えた children 本文で Default を描画し、HTML 文字列を返す。
func renderDefault(t *testing.T, lang, title, bodyHTML string) string {
	t.Helper()

	ctx := templ.WithChildren(context.Background(), templ.Raw(bodyHTML))

	var buf bytes.Buffer
	if err := Default(lang, title).Render(ctx, &buf); err != nil {
		t.Fatalf("Default(%q, %q).Render() error = %v", lang, title, err)
	}
	return buf.String()
}

func TestDefault_Japanese(t *testing.T) {
	t.Parallel()

	html := renderDefault(t, "ja", "確認用コード", "<p>BODY_MARKER</p>")

	// templ lowercases the doctype on render.
	// [Ja] templ は描画時に doctype を小文字化する。
	if !strings.Contains(html, "<!doctype html>") {
		t.Errorf("expected doctype in HTML, got: %s", html)
	}
	if !strings.Contains(html, `lang="ja"`) {
		t.Error("expected lang=ja in HTML")
	}
	if !strings.Contains(html, "<title>確認用コード</title>") {
		t.Error("expected the title in HTML")
	}
	// The children body is rendered inside the layout.
	// [Ja] children 本文がレイアウト内に描画されている。
	if !strings.Contains(html, "BODY_MARKER") {
		t.Error("expected the children body in HTML")
	}
	// The shared footer carries the Groobb signature.
	// [Ja] 共有フッターに Groobb の署名が入る。
	if !strings.Contains(html, "Groobb") {
		t.Error("expected the footer signature in HTML")
	}
}

func TestDefault_English(t *testing.T) {
	t.Parallel()

	html := renderDefault(t, "en", "Confirmation code", "<p>BODY_MARKER</p>")

	if !strings.Contains(html, `lang="en"`) {
		t.Error("expected lang=en in HTML")
	}
	if !strings.Contains(html, "<title>Confirmation code</title>") {
		t.Error("expected the title in HTML")
	}
	if !strings.Contains(html, "BODY_MARKER") {
		t.Error("expected the children body in HTML")
	}
}

func TestDefault_EscapesTitle(t *testing.T) {
	t.Parallel()

	// A title with HTML metacharacters must be escaped, not interpreted, so a
	// subject built from user-influenced text cannot inject markup.
	//
	// [Ja] HTML メタ文字を含む title は解釈されずエスケープされること。ユーザー由来の
	// テキストから組まれた件名がマークアップを注入できないようにするため。
	html := renderDefault(t, "ja", "<script>", "<p>body</p>")

	if strings.Contains(html, "<title><script></title>") {
		t.Error("title was not escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("expected the title to be HTML-escaped")
	}
}
