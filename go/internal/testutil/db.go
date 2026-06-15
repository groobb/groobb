// Package testutil provides shared helpers for tests, such as a pooled test
// database connection and per-test transaction setup.
//
// [Ja] testutil パッケージは、テスト用の共有ヘルパー (プール化したテスト DB 接続や
// テストごとのトランザクションのセットアップなど) を提供します。
package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/auth"
)

// testPool is the pgx connection pool shared across all tests in a package. It
// is established exactly once via testPoolOnce.
//
// [Ja] testPool はパッケージ内の全テストで共有する pgx 接続プールです。
// testPoolOnce によりちょうど一度だけ確立されます。
var (
	testPool     *pgxpool.Pool
	testPoolOnce sync.Once
)

// SetupTestMain initializes the shared pool eagerly from a package's TestMain
// and returns the exit code to pass to os.Exit. It is optional: SetupTx and
// GetTestDB both lazily initialize the pool, so a main_test.go is only needed
// when eager initialization is desired.
//
// Usage:
//
//	func TestMain(m *testing.M) {
//	    os.Exit(testutil.SetupTestMain(m))
//	}
//
// [Ja] SetupTestMain はパッケージの TestMain から共有プールを eager に初期化し、
// os.Exit に渡す終了コードを返します。SetupTx / GetTestDB はいずれもプールを遅延
// 初期化するため本関数は任意で、eager 初期化したい場合にのみ main_test.go を置きます。
// 使用例は上記の Usage を参照してください。
func SetupTestMain(m *testing.M) int {
	initTestPool()
	return m.Run()
}

// SetupTx begins a test transaction on the shared pool and registers a cleanup
// that rolls it back when the test finishes, isolating each test's writes. Use
// it for Repository, Validator, and Handler tests.
//
// [Ja] SetupTx は共有プール上でテスト用トランザクションを開始し、テスト終了時に
// ロールバックする cleanup を登録して各テストの書き込みを分離します。Repository /
// Validator / Handler のテストで使います。
func SetupTx(t *testing.T) (*pgxpool.Pool, pgx.Tx) {
	t.Helper()

	initTestPool()

	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("トランザクションの開始に失敗: %v", err)
	}

	t.Cleanup(func() {
		// A Rollback after a successful Commit returns pgx.ErrTxClosed, which is
		// expected for committing tests and is not a failure.
		//
		// [Ja] Commit 成功後の Rollback は pgx.ErrTxClosed を返す。コミットする
		// テストでは想定どおりで、失敗ではない。
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("トランザクションのロールバックに失敗: %v", err)
		}
	})

	return testPool, tx
}

// GetTestDB returns the shared pgx connection pool. Use it for UseCase tests
// that manage their own transactions: wrapping such a test in an outer
// transaction would hide the seeded data from the UseCase's inner transaction,
// so the data must be committed to the pool directly instead.
//
// [Ja] GetTestDB は共有 pgx 接続プールを返します。自前でトランザクションを管理する
// UseCase のテストで使います。そうしたテストを外側のトランザクションで包むと、投入
// した前提データが UseCase の内側のトランザクションから見えなくなるため、プールに
// 直接コミットする必要があります。
func GetTestDB() *pgxpool.Pool {
	initTestPool()
	return testPool
}

// DatabaseURL returns the test database connection string: DATABASE_URL if set
// (provided by the test harness), otherwise the local/CI default. It is the
// single source of truth for how tests resolve the DSN, so callers that need
// the URL string rather than the pool (such as the worker client, which takes a
// URL instead of a *pgxpool.Pool) resolve it the same way GetTestDB's pool does.
//
// [Ja] DatabaseURL はテスト DB の接続文字列を返す。DATABASE_URL (テストハーネスが
// 設定) があればそれを、無ければローカル / CI の既定値を返す。テストが DSN をどう
// 解決するかの正本であり、プールではなく URL 文字列を必要とする呼び出し元 (URL を
// 受け取る worker クライアントなど) も、GetTestDB のプールと同じ方法で解決できる。
func DatabaseURL() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	return "postgres://postgres:postgres@postgresql:5432/groobb_test?sslmode=disable"
}

// initTestPool establishes the shared pool exactly once. Whichever of
// SetupTestMain / SetupTx / GetTestDB is called first triggers it, and all
// callers share the same pool.
//
// [Ja] initTestPool は共有プールをちょうど一度だけ確立します。SetupTestMain /
// SetupTx / GetTestDB のうち最初に呼ばれたものが起点となり、すべての呼び出し元が
// 同じプールを共有します。
func initTestPool() {
	testPoolOnce.Do(func() {
		// Lower the bcrypt cost so password hashing in tests is fast
		// (DefaultCost 10 -> MinCost 4 is roughly 64x faster).
		//
		// [Ja] テストでのパスワードハッシュ化を高速化するため bcrypt コストを下げる
		// (DefaultCost 10 → MinCost 4 で約 64 倍高速)。
		auth.BcryptCost = auth.TestBcryptCost

		pool, err := pgxpool.New(context.Background(), DatabaseURL())
		if err != nil {
			panic(fmt.Sprintf("テスト用データベースへの接続に失敗: %v", err))
		}

		if err := pool.Ping(context.Background()); err != nil {
			panic(fmt.Sprintf("テスト用データベースへの ping に失敗: %v", err))
		}

		testPool = pool
	})
}
