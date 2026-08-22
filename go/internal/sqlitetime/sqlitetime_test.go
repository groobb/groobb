package sqlitetime_test

import (
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// TestTime_ValueIsFixedWidth verifies that every timestamp renders to the same
// number of characters, whatever its fractional part is. Rows are ordered by
// comparing this text, so a width that varies with the value would order them
// wrongly.
//
// [Ja] TestTime_ValueIsFixedWidth は、小数部が何であってもすべての時刻が同じ文字数に
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
			t.Fatalf("Value() error = %v", err)
		}

		text, ok := value.(string)
		if !ok {
			t.Fatalf("Value() = %T, want string", value)
		}
		if len(text) != len(sqlitetime.Layout) {
			t.Errorf("Value() for %v = %q (%d chars), want %d chars", instant, text, len(text), len(sqlitetime.Layout))
		}
	}
}

// TestTime_ValueOrdersAsText verifies that comparing two rendered timestamps as
// text puts them in the same order as the instants they denote, which is what
// SQLite relies on when it filters or sorts by a timestamp column.
//
// [Ja] TestTime_ValueOrdersAsText は、描画した 2 つの時刻をテキストとして比較したとき、
// それらが指す時点と同じ順序になることを検証します。SQLite が時刻の列で絞り込みや
// 並べ替えをするときに依拠しているのがこの性質です。
func TestTime_ValueOrdersAsText(t *testing.T) {
	t.Parallel()

	earlier, err := sqlitetime.Time(time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC)).Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	later, err := sqlitetime.Time(time.Date(2026, 8, 21, 23, 0, 0, 0, time.UTC)).Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	if earlier.(string) >= later.(string) {
		t.Errorf("%q should sort before %q", earlier, later)
	}
}

// TestTime_ValueConvertsToUTC verifies that a timestamp in another location is
// rendered as the same instant in UTC, so the stored text of two equal instants
// is identical whichever location the Go value carried.
//
// [Ja] TestTime_ValueConvertsToUTC は、別のロケーションを持つ時刻が UTC の同じ時点として
// 描画されることを検証します。これにより、等しい 2 つの時点は Go の値がどのロケーションを
// 持っていても同じテキストとして保存されます。
func TestTime_ValueConvertsToUTC(t *testing.T) {
	t.Parallel()

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("failed to load the location: %v", err)
	}

	value, err := sqlitetime.Time(time.Date(2026, 8, 21, 18, 15, 0, 0, tokyo)).Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	if want := "2026-08-21T09:15:00.000Z"; value != want {
		t.Errorf("Value() = %q, want %q", value, want)
	}
}

// TestTime_Scan verifies that a timestamp is read back from both forms it can
// arrive in: already parsed by the driver from a column declared DATETIME, and
// as text from an expression, which has no declared type to act on.
//
// [Ja] TestTime_Scan は、時刻が届きうる両方の形から読み戻せることを検証します。DATETIME と
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
				t.Fatalf("Scan() error = %v", err)
			}
			if !time.Time(got).Equal(want) {
				t.Errorf("Scan(%v) = %v, want %v", tt.src, time.Time(got), want)
			}
		})
	}
}

// TestTime_ScanRejectsUnusableValues verifies that a value the type cannot
// represent surfaces as an error rather than as a zero timestamp, so a column
// holding something other than a stored timestamp is not read as the zero time.
//
// [Ja] TestTime_ScanRejectsUnusableValues は、この型が表現できない値がゼロ値の時刻では
// なくエラーとして表れることを検証します。保存された時刻以外のものを持つ列が、ゼロ値の
// 時刻として読まれてしまうことを防ぎます。
func TestTime_ScanRejectsUnusableValues(t *testing.T) {
	t.Parallel()

	for _, src := range []any{"not a timestamp", int64(1), nil} {
		var got sqlitetime.Time
		if err := got.Scan(src); err == nil {
			t.Errorf("Scan(%v) returned no error, want one", src)
		}
	}
}

// TestPtrAndTimePtr verifies that the nullable conversions round-trip a value and
// pass nil straight through, which is what lets a caller hand an absent timestamp
// to a query without special-casing it.
//
// [Ja] TestPtrAndTimePtr は、nullable の変換が値を往復させ、nil をそのまま通すことを
// 検証します。これにより呼び出し側は、値の無い時刻を特別扱いせずクエリへ渡せます。
func TestPtrAndTimePtr(t *testing.T) {
	t.Parallel()

	if got := sqlitetime.Ptr(nil); got != nil {
		t.Errorf("Ptr(nil) = %v, want nil", got)
	}
	if got := sqlitetime.TimePtr(nil); got != nil {
		t.Errorf("TimePtr(nil) = %v, want nil", got)
	}

	want := time.Date(2026, 8, 21, 9, 15, 0, 0, time.UTC)
	got := sqlitetime.TimePtr(sqlitetime.Ptr(&want))
	if got == nil {
		t.Fatal("TimePtr(Ptr(&want)) = nil, want a timestamp")
	}
	if !got.Equal(want) {
		t.Errorf("TimePtr(Ptr(&want)) = %v, want %v", *got, want)
	}
}
