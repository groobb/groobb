package templates

import (
	"context"
	"time"
)

// Units the elapsed time is reported in, from the finest upwards. The month and
// the year are approximations (30 and 365 days), which is what a relative
// distance means at that scale: "3 months ago" is a rounding of the gap, not a
// calendar calculation, and no reader takes it for one.
//
// [Ja] 経過時間を表す単位を、細かいものから順に並べたものです。月と年は近似 (30 日と
// 365 日) です。その粒度における相対的な隔たりとは元よりそういうもので、「3 か月前」は
// 隔たりを丸めた表現であってカレンダー上の計算ではなく、読み手もそう受け取ります。
const (
	day   = 24 * time.Hour
	month = 30 * day
	year  = 365 * day
)

// RelativeTime renders an instant as the distance from now — "3 minutes ago",
// "2 days ago" — in the locale carried by ctx.
//
// A relative distance is used rather than a wall-clock date because Groobb
// resolves no time zone for the visitor: the community's pages are readable
// while signed out, so there is no account to take one from, and there is no
// setting to ask for one either. A wall-clock date would then have to be printed
// in some zone chosen for everyone, which is wrong for whoever is not in it. The
// distance between two instants is the same wherever it is read.
//
// The exact instant is not lost: MachineDateTime puts it in the datetime
// attribute of the <time> element this text sits in, so a page can start showing
// absolute times whenever a visitor's zone becomes known, without any change to
// what is stored or to the markup.
//
// An instant in the future is reported as just now rather than as a negative
// distance. It is what a clock a little ahead of the server's produces, and it
// is the nearest true thing to say about a moment that has all but arrived.
//
// [Ja] RelativeTime は、ある時点を今からの隔たり (「3 分前」「2 日前」) として、ctx が
// 運ぶロケールで描画します。
//
// 壁時計の日付ではなく相対的な隔たりを使うのは、Groobb が訪問者のタイムゾーンを解決
// しないためです。コミュニティのページはサインアウト状態でも読めるためタイムゾーンを
// 取り出すアカウントが無く、それを尋ねる設定もありません。壁時計の日付にすると、全員に
// 対して選んだ何らかのタイムゾーンで表示することになり、そこに居ない人にとっては誤った
// 時刻になります。2 つの時点の隔たりは、どこで読まれても同じです。
//
// 正確な時点が失われるわけではありません。MachineDateTime がこのテキストの入る <time>
// 要素の datetime 属性にそれを置くため、訪問者のタイムゾーンが分かるようになった時点で、
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

// MachineDateTime renders an instant for the datetime attribute of a <time>
// element: ISO 8601 in UTC, the form that attribute is defined in. It is what
// carries the exact moment while the element's text says the distance from now,
// so the two are the same fact stated for two readers.
//
// [Ja] MachineDateTime は <time> 要素の datetime 属性のために時点を描画します。書式は
// UTC の ISO 8601 で、その属性が定義されている形です。要素のテキストが今からの隔たりを
// 述べる一方、正確な時点を運ぶのがこれであり、両者は同じ事実を 2 種類の読み手に向けて
// 述べたものです。
func MachineDateTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
