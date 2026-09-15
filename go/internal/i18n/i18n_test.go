package i18n_test

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
)

func TestT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    model.Locale
		messageID string
		want      string
	}{
		{
			name:      "日本語の既定のタイトル",
			locale:    model.LocaleJa,
			messageID: "default_title",
			want:      "Groobb",
		},
		{
			name:      "英語の既定のタイトル",
			locale:    model.LocaleEn,
			messageID: "default_title",
			want:      "Groobb",
		},
		{
			name:      "日本語の既定の説明文",
			locale:    model.LocaleJa,
			messageID: "default_description",
			want:      "Groobbは掲示板サービスです。",
		},
		{
			name:      "英語の既定の説明文",
			locale:    model.LocaleEn,
			messageID: "default_description",
			want:      "Groobb is a bulletin board service.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			if got := i18n.T(ctx, tt.messageID); got != tt.want {
				t.Errorf("T(%q) = %q、期待値 = %q", tt.messageID, got, tt.want)
			}
		})
	}
}

// TestTMissingMessageは未知のメッセージIDがpanicではなくID自身に
// フォールバックすることを検証する。タイプミスが描画結果に現れるようにするため。
func TestTMissingMessage(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	const messageID = "nonexistent_message_id"
	if got := i18n.T(ctx, messageID); got != messageID {
		t.Errorf("T(%q) = %q、期待値はメッセージIDそのもの", messageID, got)
	}
}

// TestTWithTemplateDataはTがプレースホルダーデータを展開し、Countの値に
// 応じて複数形を選ぶことを検証する。Count分岐が受け付ける符号付き / 符号なしの
// 整数入力 (clampUint64ToIntのクランプを発動させる十分に大きいuint64を含む) を
// カバーし、複数形を持たない日本語がどの件数でも同じ形で描画されることも確認する。
func TestTWithTemplateData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale model.Locale
		count  any
		want   string
	}{
		{name: "英語の単数形", locale: model.LocaleEn, count: 1, want: "1 post"},
		{name: "英語の複数形", locale: model.LocaleEn, count: 5, want: "5 posts"},
		{name: "int32の件数による英語の複数形", locale: model.LocaleEn, count: int32(3), want: "3 posts"},
		{name: "int64の件数による英語の単数形", locale: model.LocaleEn, count: int64(1), want: "1 post"},
		{name: "uintの件数による英語の複数形", locale: model.LocaleEn, count: uint(2), want: "2 posts"},
		{name: "uint64の件数による英語の複数形", locale: model.LocaleEn, count: uint64(7), want: "7 posts"},
		// math.MaxIntを超えるuint64はclampUint64ToIntによってmath.MaxInt
		// にクランプされるため、複数形選択は "other" 形に解決され、描画されるCountは
		// 元の値のまま残る。
		{name: "英語では大きすぎるuint64の件数を複数形にクランプする", locale: model.LocaleEn, count: uint64(math.MaxUint64), want: "18446744073709551615 posts"},
		{name: "日本語には複数形が無い", locale: model.LocaleJa, count: 5, want: "5 件の投稿"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			got := i18n.T(ctx, "posts_count", map[string]any{"Count": tt.count})
			if got != tt.want {
				t.Errorf("T(posts_count, Count=%v) = %q、期待値 = %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestGetLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(ctx context.Context) context.Context
		want  model.Locale
	}{
		{
			name:  "日本語が設定されている",
			setup: func(ctx context.Context) context.Context { return i18n.SetLocale(ctx, model.LocaleJa) },
			want:  model.LocaleJa,
		},
		{
			name:  "英語が設定されている",
			setup: func(ctx context.Context) context.Context { return i18n.SetLocale(ctx, model.LocaleEn) },
			want:  model.LocaleEn,
		},
		{
			name:  "何も設定されていなければ既定値にフォールバックする",
			setup: func(ctx context.Context) context.Context { return ctx },
			want:  model.DefaultLocale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.setup(context.Background())
			if got := i18n.GetLocale(ctx); got != tt.want {
				t.Errorf("GetLocale() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		acceptLanguage string
		want           model.Locale
	}{
		{name: "日本語のみ", acceptLanguage: "ja", want: model.LocaleJa},
		{name: "日本語を優先", acceptLanguage: "ja,en;q=0.9", want: model.LocaleJa},
		{name: "品質値により英語を優先", acceptLanguage: "en,ja;q=0.5", want: model.LocaleEn},
		{name: "英語のみ", acceptLanguage: "en", want: model.LocaleEn},
		{name: "地域付きの英語", acceptLanguage: "en-US,en;q=0.9", want: model.LocaleEn},
		{name: "地域付きの日本語", acceptLanguage: "ja-JP", want: model.LocaleJa},
		{name: "未対応の言語", acceptLanguage: "fr,de", want: model.DefaultLocale},
		{name: "空のヘッダー", acceptLanguage: "", want: model.DefaultLocale},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}

			if got := i18n.DetectLanguage(req); got != tt.want {
				t.Errorf("DetectLanguage() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestMiddlewareはミドルウェアがAccept-Languageヘッダーから判定した
// ロケールをcontextに格納し、Tがそれを用いることを検証する。
func TestMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		acceptLanguage string
		wantLocale     model.Locale
		wantDesc       string
	}{
		{
			name:           "日本語のヘッダー",
			acceptLanguage: "ja",
			wantLocale:     model.LocaleJa,
			wantDesc:       "Groobbは掲示板サービスです。",
		},
		{
			name:           "英語のヘッダー",
			acceptLanguage: "en",
			wantLocale:     model.LocaleEn,
			wantDesc:       "Groobb is a bulletin board service.",
		},
		{
			name:           "ヘッダーが無ければ既定値にフォールバックする",
			acceptLanguage: "",
			wantLocale:     model.DefaultLocale,
			wantDesc:       "Groobbは掲示板サービスです。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotLocale model.Locale
			var gotDesc string
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotLocale = i18n.GetLocale(r.Context())
				gotDesc = i18n.T(r.Context(), "default_description")
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			rec := httptest.NewRecorder()

			i18n.Middleware(next).ServeHTTP(rec, req)

			if gotLocale != tt.wantLocale {
				t.Errorf("locale = %q、期待値 = %q", gotLocale, tt.wantLocale)
			}
			if gotDesc != tt.wantDesc {
				t.Errorf("default_description = %q、期待値 = %q", gotDesc, tt.wantDesc)
			}
		})
	}
}
