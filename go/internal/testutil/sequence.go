package testutil

import (
	"sync"
	"sync/atomic"

	"github.com/groobb/groobb/go/internal/database"
)

// sequencesはテスト用データベースごとのcounterを保持します。エントリを削除しない
// のは、あるデータベースにもう書き込まれなくなる地点がテストバイナリには無いためで、mapの
// 大きさはパッケージのテストが開くデータベースの数で頭打ちになります。
var sequences sync.Map

// nextSequenceは1つのデータベースに対する呼び出しごとに1・2・3… を返します。
// 互いに別のものに保たなければならないフィクスチャの値のためのものです。
//
// counterをテストではなくデータベースをキーにするのは、UNIQUE制約が及ぶ範囲がそれだから
// です。テストは通常自分のデータベースを所有し (SetupDBを参照)、その場合は採番が1から
// 始まるため、値は実行のたびに同じになり、失敗した検証は毎回同じ行を名指しできます。
// 1つのデータベースを開いて並行するサブテストと共有するテストでは、そのデータベースが
// 持つ1つのcounterから採番するため、サブテストどうしも衝突しません。
func nextSequence(db *database.DB) int64 {
	counter, _ := sequences.LoadOrStore(db, &atomic.Int64{})
	return counter.(*atomic.Int64).Add(1)
}
