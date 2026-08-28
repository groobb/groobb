package testutil

import (
	"sync"

	"github.com/groobb/groobb/go/internal/auth"
)

// bcryptCostOnce keeps the cost at a single write for the whole test binary.
//
// [Ja] bcryptCostOnce は、コストへの書き込みをテストバイナリ全体で 1 回に保ちます。
var bcryptCostOnce sync.Once

// LowerBcryptCost lowers the cost used to hash passwords to the minimum, which
// makes hashing in tests roughly 64 times faster (DefaultCost 10 -> MinCost 4).
//
// Every test that reaches the cost calls this rather than assigning it: the
// helpers that prepare a test database call it for the test, and a test that
// hashes without preparing one calls it itself. The cost is a package-level
// variable, so a test that hashes without going through this one sync.Once
// races with the write, whether it assigns the cost or only reads it.
//
// [Ja] LowerBcryptCost はパスワードのハッシュ化に使うコストを最小値まで下げます。
// これによりテストでのハッシュ化は約 64 倍高速になります (DefaultCost 10 →
// MinCost 4)。
//
// コストに到達するテストは、自分で代入するのではなく本関数を呼びます。テスト用
// データベースを用意するヘルパーはテストの代わりにこれを呼び、データベースを用意せずに
// ハッシュ化するテストは自分で呼びます。コストはパッケージレベルの変数であるため、この
// 1 つの sync.Once を通らずにハッシュ化するテストは、代入するかどうかにかかわらず、
// 読み取るだけでもその書き込みと競合します。
func LowerBcryptCost() {
	bcryptCostOnce.Do(func() {
		auth.BcryptCost = auth.TestBcryptCost
	})
}
