package testutil

import (
	"sync"
	"sync/atomic"

	"github.com/groobb/groobb/go/internal/database"
)

// sequences holds one counter per test database. Entries are never removed
// because a test binary has no point at which a database can no longer be
// written to, and the map is bounded by the number of databases the package's
// tests open.
//
// [Ja] sequences はテスト用データベースごとの counter を保持します。エントリを削除しない
// のは、あるデータベースにもう書き込まれなくなる地点がテストバイナリには無いためで、map の
// 大きさはパッケージのテストが開くデータベースの数で頭打ちになります。
var sequences sync.Map

// nextSequence returns 1, 2, 3, ... for successive calls against one database,
// for the fixture values that have to stay distinct from one another.
//
// The counter is keyed by database rather than by test because that is what the
// UNIQUE constraints are scoped to. A test usually owns its database (see
// SetupDB) and then numbering restarts at 1 for it, which keeps the values the
// same on every run so a failing assertion names the same row each time. A test
// that opens one database and shares it with its parallel subtests draws from the
// single counter that database owns, so those subtests cannot collide either.
//
// [Ja] nextSequence は 1 つのデータベースに対する呼び出しごとに 1・2・3… を返します。
// 互いに別のものに保たなければならないフィクスチャの値のためのものです。
//
// counter をテストではなくデータベースをキーにするのは、UNIQUE 制約が及ぶ範囲がそれだから
// です。テストは通常自分のデータベースを所有し (SetupDB を参照)、その場合は採番が 1 から
// 始まるため、値は実行のたびに同じになり、失敗した検証は毎回同じ行を名指しできます。
// 1 つのデータベースを開いて並行するサブテストと共有するテストでは、そのデータベースが
// 持つ 1 つの counter から採番するため、サブテストどうしも衝突しません。
func nextSequence(db *database.DB) int64 {
	counter, _ := sequences.LoadOrStore(db, &atomic.Int64{})
	return counter.(*atomic.Int64).Add(1)
}
