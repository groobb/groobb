package worker

import (
	"context"
	"log/slog"

	"github.com/groobb/groobb/go/internal/model"
)

// parseLocaleはジョブ引数が運ぶ言語をmodel.Localeに戻します。表示言語を
// 名指さない値にはmodel.DefaultLocaleを答え、そうしたことを記録します。
//
// キューはジョブ引数をJSONとして保持するため、ワーカーが読み戻すのは、投入した
// UseCaseが持っていた型を失った素の文字列です。これをそのまま型変換すると、
// アプリケーションが描画できる言語を持つと型が述べているフィールドへ任意の値を入れる
// ことになり、古いビルドが書いた行は、何も翻訳されていない言語を名乗ってメール
// テンプレートに届きます。フォールバックは、表示言語を名指さないAccept-Languageに
// 対してi18nが行っていることでもあり、メールを落とさず配送できます。
//
// フォールバックをログに残すのは、他に何もそれを示さないためです。メールは送られジョブは
// 成功するため、そうしなければ運用者は、自分で選んでいない言語のメールを受け取った人からしか
// これを知れません。
func parseLocale(ctx context.Context, raw string) model.Locale {
	if locale, ok := model.ParseLocale(raw); ok {
		return locale
	}

	slog.WarnContext(ctx, "ジョブ引数のロケールが表示言語を名指さないため既定ロケールで送信します", "locale", raw, "default_locale", model.DefaultLocale)

	return model.DefaultLocale
}
