package templates

import (
	"context"
	"time"
)

// 経過時間を表す単位を、細かいものから順に並べたものです。月と年は近似 (30日と
// 365日) です。その粒度における相対的な隔たりとは元よりそういうもので、「3か月前」は
// 隔たりを丸めた表現であってカレンダー上の計算ではなく、読み手もそう受け取ります。
const (
	day   = 24 * time.Hour
	month = 30 * day
	year  = 365 * day
)

// RelativeTimeは、ある時点を今からの隔たり (「3分前」「2日前」) として、ctxが
// 運ぶロケールで描画します。
//
// 壁時計の日付ではなく相対的な隔たりを使うのは、Groobbが訪問者のタイムゾーンを解決
// しないためです。コミュニティのページはサインアウト状態でも読めるためタイムゾーンを
// 取り出すアカウントが無く、それを尋ねる設定もありません。壁時計の日付にすると、全員に
// 対して選んだ何らかのタイムゾーンで表示することになり、そこに居ない人にとっては誤った
// 時刻になります。2つの時点の隔たりは、どこで読まれても同じです。
//
// 正確な時点が失われるわけではありません。MachineDateTimeがこのテキストの入る <time>
// 要素のdatetime属性にそれを置くため、訪問者のタイムゾーンが分かるようになった時点で、
// 保存しているものにもマークアップにも変更を加えずに絶対時刻の表示へ移れます。
//
// 未来の時点は負の隔たりではなく「たった今」として報告します。サーバーより少し進んだ
// 時計が生むものであり、ほぼ到達した時点について述べうる最も真に近いことだからです。
func RelativeTime(ctx context.Context, t time.Time) string {
	elapsed := time.Since(t)

	switch {
	case elapsed < time.Minute:
		return T(ctx, "datetime_just_now")
	case elapsed < time.Hour:
		return T(ctx, "datetime_minutes_ago", map[string]any{"Count": int(elapsed.Minutes())})
	case elapsed < day:
		return T(ctx, "datetime_hours_ago", map[string]any{"Count": int(elapsed.Hours())})
	case elapsed < month:
		return T(ctx, "datetime_days_ago", map[string]any{"Count": int(elapsed / day)})
	case elapsed < year:
		return T(ctx, "datetime_months_ago", map[string]any{"Count": int(elapsed / month)})
	default:
		return T(ctx, "datetime_years_ago", map[string]any{"Count": int(elapsed / year)})
	}
}

// MachineDateTimeは <time> 要素のdatetime属性のために時点を描画します。書式は
// UTCのISO 8601で、その属性が定義されている形です。要素のテキストが今からの隔たりを
// 述べる一方、正確な時点を運ぶのがこれであり、両者は同じ事実を2種類の読み手に向けて
// 述べたものです。
func MachineDateTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
