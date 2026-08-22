package testutil

import (
	"sync"

	"github.com/groobb/groobb/go/internal/auth"
)

// bcryptCostOnce keeps the cost at a single write for the whole test binary.
//
// [Ja] bcryptCostOnce は、コストへの書き込みをテストバイナリ全体で 1 回に保ちます。
var bcryptCostOnce sync.Once

// lowerBcryptCost lowers the cost used to hash passwords to the minimum, which
// makes hashing in tests roughly 64 times faster (DefaultCost 10 -> MinCost 4).
//
// Every helper that prepares a test database calls this rather than assigning
// the cost itself. The cost is a package-level variable, so two helpers
// assigning it from parallel tests would be reported by the race detector even
// though both write the same value.
//
// [Ja] lowerBcryptCost はパスワードのハッシュ化に使うコストを最小値まで下げます。
// これによりテストでのハッシュ化は約 64 倍高速になります (DefaultCost 10 →
// MinCost 4)。
//
// テスト用データベースを用意するヘルパーは、自分でコストを代入するのではなく本関数を
// 呼びます。コストはパッケージレベルの変数であるため、2 つのヘルパーが並行するテストから
// それぞれ代入すると、書き込む値が同じであっても race detector に報告されます。
func lowerBcryptCost() {
	bcryptCostOnce.Do(func() {
		auth.BcryptCost = auth.TestBcryptCost
	})
}
