package worker

import (
	"context"
	"log/slog"

	"github.com/groobb/groobb/go/internal/model"
)

// parseLocale converts the language a job argument carries back into a
// model.Locale, answering a value that names no display language with
// model.DefaultLocale and recording that it did.
//
// The queue holds job arguments as JSON, so what a worker reads back is a plain
// string that no longer carries the type the enqueueing UseCase had. Converting
// it outright would put an arbitrary value into a field whose type says it holds
// a language the application can render, and a row written by an earlier build
// would then reach a mail template naming a language nothing is translated into.
// Falling back is what i18n does for an Accept-Language header that names no
// display language, and it still delivers the mail rather than dropping it.
//
// The fallback is logged because nothing else marks it: the mail goes out and the
// job succeeds, so an operator would otherwise learn of it only from someone
// receiving mail in a language they never chose.
//
// [Ja] parseLocale はジョブ引数が運ぶ言語を model.Locale に戻します。表示言語を
// 名指さない値には model.DefaultLocale を答え、そうしたことを記録します。
//
// キューはジョブ引数を JSON として保持するため、ワーカーが読み戻すのは、投入した
// UseCase が持っていた型を失った素の文字列です。これをそのまま型変換すると、
// アプリケーションが描画できる言語を持つと型が述べているフィールドへ任意の値を入れる
// ことになり、古いビルドが書いた行は、何も翻訳されていない言語を名乗ってメール
// テンプレートに届きます。フォールバックは、表示言語を名指さない Accept-Language に
// 対して i18n が行っていることでもあり、メールを落とさず配送できます。
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
