package i18n_test

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
)

func TestT(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		locale    string
		messageID string
		want      string
	}{
		{
			name:      "default title in Japanese",
			locale:    i18n.LangJa,
			messageID: "default_title",
			want:      "Groobb",
		},
		{
			name:      "default title in English",
			locale:    i18n.LangEn,
			messageID: "default_title",
			want:      "Groobb",
		},
		{
			name:      "default description in Japanese",
			locale:    i18n.LangJa,
			messageID: "default_description",
			want:      "Groobb は掲示板サービスです。",
		},
		{
			name:      "default description in English",
			locale:    i18n.LangEn,
			messageID: "default_description",
			want:      "Groobb is a bulletin board service.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			if got := i18n.T(ctx, tt.messageID); got != tt.want {
				t.Errorf("T(%q) = %q, want %q", tt.messageID, got, tt.want)
			}
		})
	}
}

// TestTMissingMessage verifies that an unknown message ID falls back to the ID
// itself instead of panicking, so a typo is visible in the rendered output.
//
// [Ja] TestTMissingMessage は未知のメッセージ ID が panic ではなく ID 自身に
// フォールバックすることを検証する。タイプミスが描画結果に現れるようにするため。
func TestTMissingMessage(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	const messageID = "nonexistent_message_id"
	if got := i18n.T(ctx, messageID); got != messageID {
		t.Errorf("T(%q) = %q, want the message ID itself", messageID, got)
	}
}

// TestTWithTemplateData verifies that T expands placeholder data and selects the
// plural form based on the Count value. It covers the signed and unsigned integer
// inputs the Count branch accepts (including a uint64 large enough to exercise the
// clamp in clampUint64ToInt), and confirms Japanese (which has no plural) renders
// the same form for any count.
//
// [Ja] TestTWithTemplateData は T がプレースホルダーデータを展開し、Count の値に
// 応じて複数形を選ぶことを検証する。Count 分岐が受け付ける符号付き / 符号なしの
// 整数入力 (clampUint64ToInt のクランプを発動させる十分に大きい uint64 を含む) を
// カバーし、複数形を持たない日本語がどの件数でも同じ形で描画されることも確認する。
func TestTWithTemplateData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale string
		count  any
		want   string
	}{
		{name: "English singular", locale: i18n.LangEn, count: 1, want: "1 post"},
		{name: "English plural", locale: i18n.LangEn, count: 5, want: "5 posts"},
		{name: "English plural with int32 count", locale: i18n.LangEn, count: int32(3), want: "3 posts"},
		{name: "English singular with int64 count", locale: i18n.LangEn, count: int64(1), want: "1 post"},
		{name: "English plural with uint count", locale: i18n.LangEn, count: uint(2), want: "2 posts"},
		{name: "English plural with uint64 count", locale: i18n.LangEn, count: uint64(7), want: "7 posts"},
		// A uint64 above math.MaxInt is clamped to math.MaxInt by clampUint64ToInt,
		// so plural selection still resolves to "other" while the rendered Count
		// keeps the original value.
		//
		// [Ja] math.MaxInt を超える uint64 は clampUint64ToInt によって math.MaxInt
		// にクランプされるため、複数形選択は "other" 形に解決され、描画される Count は
		// 元の値のまま残る。
		{name: "English clamps oversized uint64 count to the plural form", locale: i18n.LangEn, count: uint64(math.MaxUint64), want: "18446744073709551615 posts"},
		{name: "Japanese has no plural", locale: i18n.LangJa, count: 5, want: "5 件の投稿"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), tt.locale)

			got := i18n.T(ctx, "posts_count", map[string]any{"Count": tt.count})
			if got != tt.want {
				t.Errorf("T(posts_count, Count=%v) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestGetLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(ctx context.Context) context.Context
		want  string
	}{
		{
			name:  "Japanese is set",
			setup: func(ctx context.Context) context.Context { return i18n.SetLocale(ctx, i18n.LangJa) },
			want:  i18n.LangJa,
		},
		{
			name:  "English is set",
			setup: func(ctx context.Context) context.Context { return i18n.SetLocale(ctx, i18n.LangEn) },
			want:  i18n.LangEn,
		},
		{
			name:  "nothing is set falls back to the default",
			setup: func(ctx context.Context) context.Context { return ctx },
			want:  i18n.DefaultLang,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.setup(context.Background())
			if got := i18n.GetLocale(ctx); got != tt.want {
				t.Errorf("GetLocale() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		acceptLanguage string
		want           string
	}{
		{name: "Japanese only", acceptLanguage: "ja", want: i18n.LangJa},
		{name: "Japanese preferred", acceptLanguage: "ja,en;q=0.9", want: i18n.LangJa},
		{name: "English preferred by quality value", acceptLanguage: "en,ja;q=0.5", want: i18n.LangEn},
		{name: "English only", acceptLanguage: "en", want: i18n.LangEn},
		{name: "English with region", acceptLanguage: "en-US,en;q=0.9", want: i18n.LangEn},
		{name: "Japanese with region", acceptLanguage: "ja-JP", want: i18n.LangJa},
		{name: "unsupported language", acceptLanguage: "fr,de", want: i18n.DefaultLang},
		{name: "empty header", acceptLanguage: "", want: i18n.DefaultLang},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}

			if got := i18n.DetectLanguage(req); got != tt.want {
				t.Errorf("DetectLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMiddleware verifies that the middleware stores the locale detected from
// the Accept-Language header in the context and that T uses it.
//
// [Ja] TestMiddleware はミドルウェアが Accept-Language ヘッダーから判定した
// ロケールを context に格納し、T がそれを用いることを検証する。
func TestMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		acceptLanguage string
		wantLocale     string
		wantDesc       string
	}{
		{
			name:           "Japanese header",
			acceptLanguage: "ja",
			wantLocale:     i18n.LangJa,
			wantDesc:       "Groobb は掲示板サービスです。",
		},
		{
			name:           "English header",
			acceptLanguage: "en",
			wantLocale:     i18n.LangEn,
			wantDesc:       "Groobb is a bulletin board service.",
		},
		{
			name:           "no header falls back to the default",
			acceptLanguage: "",
			wantLocale:     i18n.DefaultLang,
			wantDesc:       "Groobb は掲示板サービスです。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotLocale, gotDesc string
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
				t.Errorf("locale = %q, want %q", gotLocale, tt.wantLocale)
			}
			if gotDesc != tt.wantDesc {
				t.Errorf("default_description = %q, want %q", gotDesc, tt.wantDesc)
			}
		})
	}
}
