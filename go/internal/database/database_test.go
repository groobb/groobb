package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
)

// TestNew_InvalidURL verifies that an unparseable connection string fails at
// pool construction rather than being silently accepted.
//
// [Ja] TestNew_InvalidURL は、解析できない接続文字列がプール構築の時点で失敗し、
// 黙って受け入れられないことを検証します。
func TestNew_InvalidURL(t *testing.T) {
	t.Parallel()

	pool, err := database.New(context.Background(), "://not-a-valid-url")
	if err == nil {
		pool.Close()
		t.Fatal("expected an error for an invalid connection string, got nil")
	}
}

// TestNew_UnreachableDatabase verifies that New surfaces the startup ping
// failure when the database is unreachable, instead of returning a pool that
// only fails later on the first query.
//
// [Ja] TestNew_UnreachableDatabase は、データベースが接続不能なときに New が起動時
// ping の失敗を返し、最初のクエリで初めて失敗するプールを返さないことを検証します。
func TestNew_UnreachableDatabase(t *testing.T) {
	t.Parallel()

	// Port 1 is in the reserved range and never accepts PostgreSQL connections,
	// so the ping fails quickly. A short timeout keeps the test fast even if the
	// connection attempt would otherwise hang.
	//
	// [Ja] ポート 1 は予約された範囲で PostgreSQL 接続を受け付けないため ping は
	// すぐに失敗する。短いタイムアウトを設定することで、接続試行がハングする場合
	// でもテストを高速に保つ。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := database.New(ctx, "postgres://postgres@127.0.0.1:1/groobb_test?sslmode=disable")
	if err == nil {
		pool.Close()
		t.Fatal("expected a ping error for an unreachable database, got nil")
	}
}
