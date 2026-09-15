package sqlitetime_test

import (
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// TestTime_ValueIsFixedWidthは、小数部が何であってもすべての時刻が同じ文字数に
// なることを検証します。行の順序はこのテキストの比較で決まるため、値によって幅が変わると
// 順序が壊れます。
func TestTime_ValueIsFixedWidth(t *testing.T) {
	t.Parallel()

	instants := []time.Time{
		time.Date(2026, 8, 21, 9, 15, 0, 0, time.UTC),
		time.Date(2026, 8, 21, 9, 15, 0, 970*int(time.Millisecond), time.UTC),
		time.Date(2026, 8, 21, 9, 15, 0, 7*int(time.Millisecond), time.UTC),
		time.Date(2026, 12, 31, 23, 59, 59, 999*int(time.Millisecond), time.UTC),
	}

	for _, instant := range instants {
		value, err := sqlitetime.Time(instant).Value()
		if err != nil {
			t.Fatalf("Value()のエラー = %v", err)
		}

		text, ok := value.(string)
		if !ok {
			t.Fatalf("Value()の型 = %T、期待値 = string", value)
		}
		if len(text) != len(sqlitetime.Layout) {
			t.Errorf("%v に対するValue() = %q (%d 文字)、期待値 = %d 文字", instant, text, len(text), len(sqlitetime.Layout))
		}
	}
}

// TestTime_ValueOrdersAsTextは、描画した2つの時刻をテキストとして比較したとき、
// それらが指す時点と同じ順序になることを検証します。SQLiteが時刻の列で絞り込みや
// 並べ替えをするときに依拠しているのがこの性質です。
func TestTime_ValueOrdersAsText(t *testing.T) {
	t.Parallel()

	earlier, err := sqlitetime.Time(time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC)).Value()
	if err != nil {
		t.Fatalf("Value()のエラー = %v", err)
	}
	later, err := sqlitetime.Time(time.Date(2026, 8, 21, 23, 0, 0, 0, time.UTC)).Value()
	if err != nil {
		t.Fatalf("Value()のエラー = %v", err)
	}

	if earlier.(string) >= later.(string) {
		t.Errorf("%q が %q より前に並ばない", earlier, later)
	}
}

// TestTime_ValueConvertsToUTCは、別のロケーションを持つ時刻がUTCの同じ時点として
// 描画されることを検証します。これにより、等しい2つの時点はGoの値がどのロケーションを
// 持っていても同じテキストとして保存されます。
func TestTime_ValueConvertsToUTC(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("ロケーションの読み込みに失敗: %v", err)
	}

	value, err := sqlitetime.Time(time.Date(2026, 8, 21, 18, 15, 0, 0, tokyo)).Value()
	if err != nil {
		t.Fatalf("Value()のエラー = %v", err)
	}

	if want := "2026-08-21T09:15:00.000Z"; value != want {
		t.Errorf("Value() = %q、期待値 = %q", value, want)
	}
}

// TestTime_Scanは、時刻が届きうる両方の形から読み戻せることを検証します。DATETIMEと
// 宣言された列からドライバが既に解釈した形と、手掛かりになる宣言型を持たない式から届く
// テキストの形です。
func TestTime_Scan(t *testing.T) {
	t.Parallel()

	want := time.Date(2026, 8, 21, 9, 15, 0, 970*int(time.Millisecond), time.UTC)

	tests := []struct {
		name string
		src  any
	}{
		{name: "time.Time", src: want},
		{name: "string", src: "2026-08-21T09:15:00.970Z"},
		{name: "bytes", src: []byte("2026-08-21T09:15:00.970Z")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got sqlitetime.Time
			if err := got.Scan(tt.src); err != nil {
				t.Fatalf("Scan()のエラー = %v", err)
			}
			if !time.Time(got).Equal(want) {
				t.Errorf("Scan(%v) = %v、期待値 = %v", tt.src, time.Time(got), want)
			}
		})
	}
}

// TestTime_ScanRejectsUnusableValuesは、この型が表現できない値がゼロ値の時刻では
// なくエラーとして表れることを検証します。保存された時刻以外のものを持つ列が、ゼロ値の
// 時刻として読まれてしまうことを防ぎます。
func TestTime_ScanRejectsUnusableValues(t *testing.T) {
	t.Parallel()

	for _, src := range []any{"not a timestamp", int64(1), nil} {
		var got sqlitetime.Time
		if err := got.Scan(src); err == nil {
			t.Errorf("Scan(%v)のエラー = nil、エラーを期待", src)
		}
	}
}

// TestPtrAndTimePtrは、nullableの変換が値を往復させ、nilをそのまま通すことを
// 検証します。これにより呼び出し側は、値の無い時刻を特別扱いせずクエリへ渡せます。
func TestPtrAndTimePtr(t *testing.T) {
	t.Parallel()

	if got := sqlitetime.Ptr(nil); got != nil {
		t.Errorf("Ptr(nil) = %v、期待値 = nil", got)
	}
	if got := sqlitetime.TimePtr(nil); got != nil {
		t.Errorf("TimePtr(nil) = %v、期待値 = nil", got)
	}

	want := time.Date(2026, 8, 21, 9, 15, 0, 0, time.UTC)
	got := sqlitetime.TimePtr(sqlitetime.Ptr(&want))
	if got == nil {
		t.Fatal("TimePtr(Ptr(&want)) = nil、時刻を期待")
	}
	if !got.Equal(want) {
		t.Errorf("TimePtr(Ptr(&want)) = %v、期待値 = %v", *got, want)
	}
}
