// i18nパッケージは国際化機能を提供します。リクエストからのロケール判定、
// go-i18nを用いた翻訳関数、解決したロケールをリクエストcontextに格納するHTTP
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

	"github.com/groobb/groobb/go/internal/model"
)

// ロケールファイルを埋め込み、バイナリを自己完結させて実行時に外部の翻訳
// ファイルを必要としないようにする。
//
//go:embed locales/*.toml
var localesFS embed.FS

// contextKeyはcontextキー用の非公開型で、他パッケージで定義されたキーとの
// 衝突を避けるために用いる。
type contextKey string

const (
	localeContextKey    contextKey = "locale"
	localizerContextKey contextKey = "localizer"
)

// bundleはサポートする全言語のパース済み翻訳を保持する。起動時に一度だけ
// 構築し、以降は読み取り専用で扱う。
var bundle *i18n.Bundle

// initは埋め込まれたロケールファイルから翻訳バンドルを構築する。
func init() {
	bundle = i18n.NewBundle(language.Japanese)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, locale := range model.Locales() {
		data, err := localesFS.ReadFile(fmt.Sprintf("locales/%s.toml", locale))
		if err != nil {
			// ロケールファイルはビルド時に埋め込まれるため、読み込み失敗は
			// ファイル名とmodel.Locales() のロケールの不整合 (リネーム漏れや、ファイルを
			// 伴わない言語の追加など) を意味する。該当ロケールが欠けたまま黙って起動せず、
			// fail-fastする。
			panic(fmt.Sprintf("i18n: failed to read embedded locale file locales/%s.toml: %v", locale, err))
		}

		bundle.MustParseMessageFileBytes(data, fmt.Sprintf("%s.toml", locale))
	}
}

// Tはctxに格納されたロケールでmessageIDを翻訳する。翻訳が見つからない
// 場合はmessageIDをそのまま返すため、タイプミスはクラッシュではなく描画結果に
// 現れる。
func T(ctx context.Context, messageID string, templateData ...map[string]any) string {
	localizer := GetLocalizer(ctx)

	config := &i18n.LocalizeConfig{
		MessageID: messageID,
	}

	if len(templateData) > 0 && templateData[0] != nil {
		config.TemplateData = templateData[0]

		// Countが渡された場合は複数形処理を有効にする。
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

// pluralCountは任意のCount値を複数形選択用のintに変換する。符号付き /
// 符号なしのいずれの整数型も受け付けるため、呼び出し元はCountがint32のカラム
// 由来かint64のCOUNT(*) 由来かを気にしなくてよい。これが無いとint / int32
// 以外のcountではPluralCountが未設定のまま "other" 形に黙ってフォールバック
// する (例: "1 posts")。第2戻り値はvが整数だったかどうかを表す。
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

// clampUint64ToIntは符号なしのcountをintに変換し、math.MaxIntを超える
// 値はクランプする。複数形選択は「1かどうか」しか区別しないため、それほど大きな
// countをmath.MaxIntにクランプしても "other" 形に解決され、負数へのオーバー
// フローを避けられる。
func clampUint64ToInt(v uint64) int {
	if v > math.MaxInt {
		return math.MaxInt
	}
	return int(v)
}

// GetLocaleはctxに格納されたロケールを返す。未設定の場合は
// model.DefaultLocaleを返す。
func GetLocale(ctx context.Context) model.Locale {
	if locale, ok := ctx.Value(localeContextKey).(model.Locale); ok {
		return locale
	}
	return model.DefaultLocale
}

// SetLocaleは指定したロケールを格納したctxのコピーを返す。
func SetLocale(ctx context.Context, locale model.Locale) context.Context {
	return context.WithValue(ctx, localeContextKey, locale)
}

// GetLocalizerはctxに格納されたLocalizerを返す。無い場合はcontextの
// ロケールから生成するため、ミドルウェア無し (例: SetLocaleだけを呼ぶテスト) でも
// Tが機能する。
func GetLocalizer(ctx context.Context) *i18n.Localizer {
	if localizer, ok := ctx.Value(localizerContextKey).(*i18n.Localizer); ok {
		return localizer
	}
	return i18n.NewLocalizer(bundle, string(GetLocale(ctx)))
}

// SetLocalizerは指定したLocalizerを格納したctxのコピーを返す。
func SetLocalizer(ctx context.Context, localizer *i18n.Localizer) context.Context {
	return context.WithValue(ctx, localizerContextKey, localizer)
}

// DetectLanguageはリクエストのAccept-Languageヘッダーから表示言語を選ぶ。
// ParseAcceptLanguageは要求言語を品質値順 (優先度の高い順) で返すため、
// model.ParseLocaleが受け付ける最初のものを返すことでクライアントの優先順を尊重
// できる。認識できないものはmodel.DefaultLocaleにフォールバックする。
//
// ここではlanguage.Matcherを意図的に使わない。表示言語が2つだけだと、その
// 言語距離ヒューリスティックが想定外の挙動をする (例: 未対応の "de" を英語に
// マップする) ためで、パース済みタグを明示的に走査する方が予測可能。
func DetectLanguage(r *http.Request) model.Locale {
	tags, _, _ := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))

	for _, tag := range tags {
		base, _ := tag.Base()
		if locale, ok := model.ParseLocale(base.String()); ok {
			return locale
		}
	}

	return model.DefaultLocale
}

// MiddlewareはAccept-Languageヘッダーからリクエストのロケールを解決し、
// 後続のハンドラーやテンプレートが参照できるよう、ロケールと対応するLocalizerの
// 両方をリクエストcontextに格納する。
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := DetectLanguage(r)
		localizer := i18n.NewLocalizer(bundle, string(locale))

		ctx := SetLocale(r.Context(), locale)
		ctx = SetLocalizer(ctx, localizer)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
