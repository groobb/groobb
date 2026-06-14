package testutil_test

import (
	"context"
	"os"
	"testing"

	"github.com/groobb/groobb/go/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// TestGetTestDB verifies that the shared pool is returned and is reachable.
//
// [Ja] TestGetTestDB は共有プールが返り、かつ疎通できることを検証します。
func TestGetTestDB(t *testing.T) {
	t.Parallel()

	pool := testutil.GetTestDB()
	if pool == nil {
		t.Fatal("GetTestDB() returned nil")
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

// TestSetupTx verifies that SetupTx returns a usable transaction on the shared
// pool.
//
// [Ja] TestSetupTx は SetupTx が共有プール上の利用可能なトランザクションを返すこと
// を検証します。
func TestSetupTx(t *testing.T) {
	t.Parallel()

	pool, tx := testutil.SetupTx(t)
	if pool == nil {
		t.Fatal("SetupTx() returned a nil pool")
	}
	if tx == nil {
		t.Fatal("SetupTx() returned a nil tx")
	}

	var got int
	if err := tx.QueryRow(context.Background(), "SELECT 1").Scan(&got); err != nil {
		t.Fatalf("querying inside the transaction error = %v", err)
	}
	if got != 1 {
		t.Errorf("SELECT 1 = %d, want 1", got)
	}
}

// TestSetupTxMultipleTransactions verifies that multiple SetupTx calls share the
// same pool while returning independent transactions.
//
// [Ja] TestSetupTxMultipleTransactions は複数回の SetupTx 呼び出しが同じプールを
// 共有しつつ、独立したトランザクションを返すことを検証します。
func TestSetupTxMultipleTransactions(t *testing.T) {
	t.Parallel()

	pool1, tx1 := testutil.SetupTx(t)
	pool2, tx2 := testutil.SetupTx(t)

	if pool1 != pool2 {
		t.Error("SetupTx() should return the same shared pool")
	}

	var r1, r2 int
	if err := tx1.QueryRow(context.Background(), "SELECT 1").Scan(&r1); err != nil {
		t.Fatalf("tx1 query error = %v", err)
	}
	if err := tx2.QueryRow(context.Background(), "SELECT 2").Scan(&r2); err != nil {
		t.Fatalf("tx2 query error = %v", err)
	}
	if r1 != 1 || r2 != 2 {
		t.Errorf("got r1=%d r2=%d, want r1=1 r2=2", r1, r2)
	}
}
