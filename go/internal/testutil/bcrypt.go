package testutil

import (
	"sync"

	"github.com/groobb/groobb/go/internal/auth"
)

// bcryptCostOnceは、コストへの書き込みをテストバイナリ全体で1回に保ちます。
var bcryptCostOnce sync.Once

// LowerBcryptCostはパスワードのハッシュ化に使うコストを最小値まで下げます。
// これによりテストでのハッシュ化は約64倍高速になります (DefaultCost 10 →
// MinCost 4)。
//
// コストに到達するテストは、自分で代入するのではなく本関数を呼びます。テスト用
// データベースを用意するヘルパーはテストの代わりにこれを呼び、データベースを用意せずに
// ハッシュ化するテストは自分で呼びます。コストはパッケージレベルの変数であるため、この
// 1つのsync.Onceを通らずにハッシュ化するテストは、代入するかどうかにかかわらず、
// 読み取るだけでもその書き込みと競合します。
func LowerBcryptCost() {
	bcryptCostOnce.Do(func() {
		auth.BcryptCost = auth.TestBcryptCost
	})
}
