// sqlitetimeパッケージは、GroobbがSQLiteに保存する時刻の型を定義します。
//
// SQLiteに日付専用の型は無く、時刻はテキストで、行の順序はそのテキストの比較で
// 決まります。そのためGroobbはすべての時刻を1つの固定書式で書き込み、本パッケージが
// すべての値をその書式に従わせます。
package sqlitetime

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Layoutはすべての時刻を保存する書式です。桁数を3桁に固定した小数部を持つ
// ISO8601 UTCで、各列のstrftimeの既定値が生成するものと同一です。すべての要素の
// 幅が一定になるようゼロ詰めするため、2つの時刻をテキストとして比較すると時点として
// 順序付きます。幅の揺れる書式 (末尾のゼロが落ちる、小数部が省かれる) では順序が
// 壊れます。
const Layout = "2006-01-02T15:04:05.000Z"

// TimeはSQLiteとの境界をLayoutで往復するtime.Timeです。
//
// 素のtime.Timeをパラメータとして渡すと、ドライバはGoのtime.Time.Stringの書式で
// 束縛します。この書式は日付と時刻を空白で区切りますが、保存されているテキストの区切りは
// "T" です。両者をテキストとして比較すると、束縛した値は列の既定値が書いた行に対して
// 常に誤った側に並び、しかもエラーにならないため、時刻で範囲を区切るクエリが黙って
// 誤った行を返します。時刻をこの型で渡すことが、両側の書式を1つに保つ手段です。
type Time time.Time

// Ptrはnullableな時刻をクエリへ渡す方向で変換します。nilにはnilを返すため、
// 呼び出し側は値が無い場合を特別扱いする必要がありません。
func Ptr(t *time.Time) *Time {
	if t == nil {
		return nil
	}
	converted := Time(*t)
	return &converted
}

// TimePtrはnullableな時刻をクエリの行から取り出す方向で変換します。nilには
// nilを返すため、呼び出し側は値が無い場合を特別扱いする必要がありません。
func TimePtr(t *Time) *time.Time {
	if t == nil {
		return nil
	}
	converted := time.Time(*t)
	return &converted
}

// Valueは時刻をSQLiteが保存するテキストとしてUTCで表現します。別の
// ロケーションを持つ値は拒否せず変換します。ロケーションはGoの値の性質であって、
// それが指す時点の性質ではないためです。
func (t Time) Value() (driver.Value, error) {
	return time.Time(t).UTC().Format(Layout), nil
}

// Scanはクエリの行から時刻を読み戻します。
//
// ドライバは列が何を返すかをその宣言型から決めるため、DATETIMEと宣言された列は
// 既にtime.Timeとして解釈された状態で届きます。テキストも受け付けます。式を経由して
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

// scanTextは保存されたテキストを時刻に解釈し直します。
func (t *Time) scanText(s string) error {
	parsed, err := time.Parse(Layout, s)
	if err != nil {
		return fmt.Errorf("failed to parse %q as a timestamp: %w", s, err)
	}
	*t = Time(parsed)
	return nil
}
