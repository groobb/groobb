// Package i18n provides internationalization: locale detection from requests,
// a translation function backed by go-i18n, and an HTTP middleware that stores
// the resolved locale in the request context.
//
// [Ja] i18n パッケージは国際化機能を提供します。リクエストからのロケール判定、
// go-i18n を用いた翻訳関数、解決したロケールをリクエスト context に格納する HTTP
// ミドルウェアを含みます。
package i18n

import (
	"context"
	"embed"
	"fmt"
	"math"
	"net/http"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Embed the locale files so the binary is self-contained and needs no external
// translation files at runtime.
//
// [Ja] ロケールファイルを埋め込み、バイナリを自己完結させて実行時に外部の翻訳
// ファイルを必要としないようにする。
//
//go:embed locales/*.toml
var localesFS embed.FS

// Supported languages. Japanese is the default.
// [Ja] サポートする言語。日本語をデフォルトとする。
const (
	LangJa      = "ja"
	LangEn      = "en"
	DefaultLang = LangJa
)

// contextKey is an unexported type for context keys to avoid collisions with
// keys defined in other packages.
//
// [Ja] contextKey は context キー用の非公開型で、他パッケージで定義されたキーとの
// 衝突を避けるために用いる。
type contextKey string

const (
	localeContextKey    contextKey = "locale"
	localizerContextKey contextKey = "localizer"
)

// bundle holds the parsed translations for every supported language. It is
// built once at startup and only read afterwards.
//
// [Ja] bundle はサポートする全言語のパース済み翻訳を保持する。起動時に一度だけ
// 構築し、以降は読み取り専用で扱う。
var bundle *i18n.Bundle

// init builds the translation bundle from the embedded locale files.
// [Ja] init は埋め込まれたロケールファイルから翻訳バンドルを構築する。
func init() {
	bundle = i18n.NewBundle(language.Japanese)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, code := range []string{LangJa, LangEn} {
		data, err := localesFS.ReadFile(fmt.Sprintf("locales/%s.toml", code))
		if err != nil {
			// The locale files are embedded at build time, so a read failure
			// means the file name and the hard-coded code are out of sync
			// (e.g. a renamed file). Fail fast instead of silently starting
			// with that locale missing.
			//
			// [Ja] ロケールファイルはビルド時に埋め込まれるため、読み込み失敗は
			// ファイル名とハードコードしたコードの不整合 (リネーム漏れ等) を意味する。
			// 該当ロケールが欠けたまま黙って起動せず、fail-fast する。
			panic(fmt.Sprintf("i18n: failed to read embedded locale file locales/%s.toml: %v", code, err))
		}

		bundle.MustParseMessageFileBytes(data, fmt.Sprintf("%s.toml", code))
	}
}

// T translates messageID using the locale stored in ctx. When the translation
// is missing it falls back to returning messageID, so a typo surfaces in the
// rendered output instead of crashing.
//
// [Ja] T は ctx に格納されたロケールで messageID を翻訳する。翻訳が見つからない
// 場合は messageID をそのまま返すため、タイプミスはクラッシュではなく描画結果に
// 現れる。
func T(ctx context.Context, messageID string, templateData ...map[string]any) string {
	localizer := GetLocalizer(ctx)

	config := &i18n.LocalizeConfig{
		MessageID: messageID,
	}

	if len(templateData) > 0 && templateData[0] != nil {
		config.TemplateData = templateData[0]

		// Enable plural handling when a Count value is supplied.
		// [Ja] Count が渡された場合は複数形処理を有効にする。
		if count, ok := pluralCount(templateData[0]["Count"]); ok {
			config.PluralCount = count
		}
	}

	message, err := localizer.Localize(config)
	if err != nil {
		return messageID
	}

	return message
}

// pluralCount converts an arbitrary Count value into an int for plural
// selection. It accepts every signed and unsigned integer type so callers
// don't have to care whether the count comes from a typed int32 column, an
// int64 COUNT(*), etc.; without this a non-int/int32 count would leave
// PluralCount unset and silently fall back to the "other" form (e.g. "1
// posts"). The second return value reports whether v was an integer.
//
// [Ja] pluralCount は任意の Count 値を複数形選択用の int に変換する。符号付き /
// 符号なしのいずれの整数型も受け付けるため、呼び出し元は Count が int32 のカラム
// 由来か int64 の COUNT(*) 由来かを気にしなくてよい。これが無いと int / int32
// 以外の count では PluralCount が未設定のまま "other" 形に黙ってフォールバック
// する (例: "1 posts")。第 2 戻り値は v が整数だったかどうかを表す。
func pluralCount(v any) (int, bool) {
	switch count := v.(type) {
	case int:
		return count, true
	case int8:
		return int(count), true
	case int16:
		return int(count), true
	case int32:
		return int(count), true
	case int64:
		return int(count), true
	case uint:
		return clampUint64ToInt(uint64(count)), true
	case uint8:
		return int(count), true
	case uint16:
		return int(count), true
	case uint32:
		return int(count), true
	case uint64:
		return clampUint64ToInt(count), true
	default:
		return 0, false
	}
}

// clampUint64ToInt converts an unsigned count to int, clamping values that
// exceed math.MaxInt. Plural selection only distinguishes "is it 1?" from
// "anything else", so clamping a count that large to math.MaxInt still
// resolves to the "other" form and avoids an overflow wraparound to a
// negative number.
//
// [Ja] clampUint64ToInt は符号なしの count を int に変換し、math.MaxInt を超える
// 値はクランプする。複数形選択は「1 かどうか」しか区別しないため、それほど大きな
// count を math.MaxInt にクランプしても "other" 形に解決され、負数へのオーバー
// フローを避けられる。
func clampUint64ToInt(v uint64) int {
	if v > math.MaxInt {
		return math.MaxInt
	}
	return int(v)
}

// GetLocale returns the locale stored in ctx, or the default language when none
// is set.
//
// [Ja] GetLocale は ctx に格納されたロケールを返す。未設定の場合はデフォルト言語を
// 返す。
func GetLocale(ctx context.Context) string {
	if locale, ok := ctx.Value(localeContextKey).(string); ok {
		return locale
	}
	return DefaultLang
}

// SetLocale returns a copy of ctx with the given locale stored in it.
// [Ja] SetLocale は指定したロケールを格納した ctx のコピーを返す。
func SetLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeContextKey, locale)
}

// GetLocalizer returns the Localizer stored in ctx. When none is present it
// builds one from the context locale, so T works even without the middleware
// (e.g. in tests that only call SetLocale).
//
// [Ja] GetLocalizer は ctx に格納された Localizer を返す。無い場合は context の
// ロケールから生成するため、ミドルウェア無し (例: SetLocale だけを呼ぶテスト) でも
// T が機能する。
func GetLocalizer(ctx context.Context) *i18n.Localizer {
	if localizer, ok := ctx.Value(localizerContextKey).(*i18n.Localizer); ok {
		return localizer
	}
	return i18n.NewLocalizer(bundle, GetLocale(ctx))
}

// SetLocalizer returns a copy of ctx with the given Localizer stored in it.
// [Ja] SetLocalizer は指定した Localizer を格納した ctx のコピーを返す。
func SetLocalizer(ctx context.Context, localizer *i18n.Localizer) context.Context {
	return context.WithValue(ctx, localizerContextKey, localizer)
}

// DetectLanguage picks a supported language from the request's Accept-Language
// header. ParseAcceptLanguage returns the requested languages sorted by quality
// value (most preferred first), so returning the first one we support honors the
// client's preference order. Anything unrecognized falls back to the default
// language.
//
// We deliberately avoid language.Matcher here: with only two supported languages
// its language-distance heuristics surprise us (e.g. it maps an unsupported "de"
// onto English), whereas an explicit scan over the parsed tags is predictable.
//
// [Ja] DetectLanguage はリクエストの Accept-Language ヘッダーからサポート対象の
// 言語を選ぶ。ParseAcceptLanguage は要求言語を品質値順 (優先度の高い順) で返すため、
// 最初にサポートしている言語を返すことでクライアントの優先順を尊重できる。認識
// できないものはデフォルト言語にフォールバックする。
//
// ここでは language.Matcher を意図的に使わない。サポート言語が 2 つだけだと、その
// 言語距離ヒューリスティックが想定外の挙動をする (例: 未対応の "de" を英語に
// マップする) ためで、パース済みタグを明示的に走査する方が予測可能。
func DetectLanguage(r *http.Request) string {
	tags, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))

	for _, tag := range tags {
		base, _ := tag.Base()
		switch base.String() {
		case LangJa:
			return LangJa
		case LangEn:
			return LangEn
		}
	}

	return DefaultLang
}

// Middleware resolves the request locale from the Accept-Language header and
// stores both the locale and a matching Localizer in the request context for
// downstream handlers and templates.
//
// [Ja] Middleware は Accept-Language ヘッダーからリクエストのロケールを解決し、
// 後続のハンドラーやテンプレートが参照できるよう、ロケールと対応する Localizer の
// 両方をリクエスト context に格納する。
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := DetectLanguage(r)
		localizer := i18n.NewLocalizer(bundle, locale)

		ctx := SetLocale(r.Context(), locale)
		ctx = SetLocalizer(ctx, localizer)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
