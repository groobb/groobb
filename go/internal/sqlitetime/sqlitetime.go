// Package sqlitetime defines the timestamp type Groobb stores in SQLite.
//
// SQLite has no dedicated date type: a timestamp is text, and rows are ordered
// by comparing that text. Groobb therefore writes every timestamp in one fixed
// format, and this package is what holds every value to it.
//
// [Ja] sqlitetime パッケージは、Groobb が SQLite に保存する時刻の型を定義します。
//
// SQLite に日付専用の型は無く、時刻はテキストで、行の順序はそのテキストの比較で
// 決まります。そのため Groobb はすべての時刻を 1 つの固定書式で書き込み、本パッケージが
// すべての値をその書式に従わせます。
package sqlitetime

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Layout is the format every timestamp is stored in: ISO8601 UTC with a fixed
// three-digit fraction, identical to what the columns' strftime defaults
// produce. Every field is zero-padded to a constant width so that comparing two
// timestamps as text orders them as instants; a format whose width varies (a
// dropped trailing zero, an omitted fraction) would order wrongly.
//
// [Ja] Layout はすべての時刻を保存する書式です。桁数を 3 桁に固定した小数部を持つ
// ISO8601 UTC で、各列の strftime の既定値が生成するものと同一です。すべての要素の
// 幅が一定になるようゼロ詰めするため、2 つの時刻をテキストとして比較すると時点として
// 順序付きます。幅の揺れる書式 (末尾のゼロが落ちる、小数部が省かれる) では順序が
// 壊れます。
const Layout = "2006-01-02T15:04:05.000Z"

// Time is a time.Time that crosses the SQLite boundary in Layout.
//
// The driver binds a plain time.Time parameter in Go's own time.Time.String
// format, whose date and time are separated by a space where the stored text
// has a "T". Comparing the two as text puts every bound value on the wrong side
// of the rows written by the column defaults, and does so without any error, so
// a query bounded by a timestamp silently returns the wrong rows. Passing
// timestamps as this type instead is what keeps both sides in one format.
//
// [Ja] Time は SQLite との境界を Layout で往復する time.Time です。
//
// 素の time.Time をパラメータとして渡すと、ドライバは Go の time.Time.String の書式で
// 束縛します。この書式は日付と時刻を空白で区切りますが、保存されているテキストの区切りは
// "T" です。両者をテキストとして比較すると、束縛した値は列の既定値が書いた行に対して
// 常に誤った側に並び、しかもエラーにならないため、時刻で範囲を区切るクエリが黙って
// 誤った行を返します。時刻をこの型で渡すことが、両側の書式を 1 つに保つ手段です。
type Time time.Time

// Ptr converts a nullable timestamp on its way into a query. It returns nil for
// nil, so a caller does not have to special-case the absent value.
//
// [Ja] Ptr は nullable な時刻をクエリへ渡す方向で変換します。nil には nil を返すため、
// 呼び出し側は値が無い場合を特別扱いする必要がありません。
func Ptr(t *time.Time) *Time {
	if t == nil {
		return nil
	}
	converted := Time(*t)
	return &converted
}

// TimePtr converts a nullable timestamp on its way out of a query row. It
// returns nil for nil, so a caller does not have to special-case the absent
// value.
//
// [Ja] TimePtr は nullable な時刻をクエリの行から取り出す方向で変換します。nil には
// nil を返すため、呼び出し側は値が無い場合を特別扱いする必要がありません。
func TimePtr(t *Time) *time.Time {
	if t == nil {
		return nil
	}
	converted := time.Time(*t)
	return &converted
}

// Value renders the timestamp as the text SQLite stores, in UTC. A value in
// another location is converted rather than rejected: the location is a
// property of the Go value, not of the instant it denotes.
//
// [Ja] Value は時刻を SQLite が保存するテキストとして UTC で表現します。別の
// ロケーションを持つ値は拒否せず変換します。ロケーションは Go の値の性質であって、
// それが指す時点の性質ではないためです。
func (t Time) Value() (driver.Value, error) {
	return time.Time(t).UTC().Format(Layout), nil
}

// Scan reads a timestamp back from a query row.
//
// The driver decides what a column yields from its declared type, so a column
// declared DATETIME arrives already parsed as a time.Time. Text is accepted as
// well, for a value read through an expression, which has no declared type for
// the driver to act on.
//
// [Ja] Scan はクエリの行から時刻を読み戻します。
//
// ドライバは列が何を返すかをその宣言型から決めるため、DATETIME と宣言された列は
// 既に time.Time として解釈された状態で届きます。テキストも受け付けます。式を経由して
// 読んだ値には、ドライバが手掛かりにする宣言型が無いためです。
func (t *Time) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		*t = Time(v.UTC())
		return nil
	case string:
		return t.scanText(v)
	case []byte:
		return t.scanText(string(v))
	default:
		return fmt.Errorf("cannot scan %T into a sqlitetime.Time", src)
	}
}

// scanText parses stored text back into a timestamp.
//
// [Ja] scanText は保存されたテキストを時刻に解釈し直します。
func (t *Time) scanText(s string) error {
	parsed, err := time.Parse(Layout, s)
	if err != nil {
		return fmt.Errorf("failed to parse %q as a timestamp: %w", s, err)
	}
	*t = Time(parsed)
	return nil
}
