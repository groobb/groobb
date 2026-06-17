package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newPasswordResetTokenRepo builds a PasswordResetTokenRepository bound to the
// test transaction, exercising WithTx so writes roll back when the test
// finishes. It returns the transaction too so a test can seed a user (the FK
// target) and count rows directly.
//
// [Ja] newPasswordResetTokenRepo はテスト用トランザクションに束ねた
// PasswordResetTokenRepository を作る。WithTx を通すことで、テスト終了時に書き込みが
// ロールバックされる。テストが (FK の対象となる) ユーザーを仕込み、行を直接数えられるよう
// トランザクションも返す。
func newPasswordResetTokenRepo(t *testing.T) (*repository.PasswordResetTokenRepository, pgx.Tx, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	return repository.NewPasswordResetTokenRepository(query.New(db)).WithTx(tx), tx, context.Background()
}

// countTokens returns how many password_reset_tokens rows exist for the user in
// the test transaction.
//
// [Ja] countTokens はテスト用トランザクション内で、そのユーザーの
// password_reset_tokens 行数を返す。
func countTokens(t *testing.T, tx pgx.Tx, ctx context.Context, userID model.UserID) int {
	t.Helper()
	var count int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = $1", uuid.UUID(userID)).Scan(&count); err != nil {
		t.Fatalf("トークン数の取得に失敗: %v", err)
	}
	return count
}

// TestPasswordResetTokenRepository_Create verifies that Create persists a token
// and returns it with the database-assigned id and timestamps, the stored digest
// and expiry, and a nil used_at (a freshly issued token is unused).
//
// [Ja] TestPasswordResetTokenRepository_Create は、Create がトークンを永続化し、DB が
// 採番した id とタイムスタンプ・保存したダイジェストと有効期限・nil の used_at (発行直後の
// トークンは未使用) を伴って返すことを検証する。
func TestPasswordResetTokenRepository_Create(t *testing.T) {
	t.Parallel()

	repo, tx, ctx := newPasswordResetTokenRepo(t)
	userID := testutil.NewUserBuilder(t, tx).Build()

	expiresAt := time.Now().Add(model.PasswordResetTokenExpirationDuration)
	token, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "digest-abc",
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if token.ID == (model.PasswordResetTokenID{}) {
		t.Error("Create() token.ID は DB 採番で空でないはず")
	}
	if token.UserID != userID {
		t.Errorf("token.UserID = %v, want %v", token.UserID, userID)
	}
	if token.TokenDigest != "digest-abc" {
		t.Errorf("token.TokenDigest = %q, want %q", token.TokenDigest, "digest-abc")
	}
	// Postgres timestamptz keeps microsecond precision, so the round-tripped value
	// can differ from the nanosecond-precision input by up to a microsecond;
	// compare within that tolerance rather than for exact equality.
	//
	// [Ja] Postgres の timestamptz はマイクロ秒精度のため、往復した値はナノ秒精度の入力と
	// 最大 1 マイクロ秒ずれうる。完全一致ではなくその許容差で比較する。
	if diff := token.ExpiresAt.Sub(expiresAt); diff < -time.Microsecond || diff > time.Microsecond {
		t.Errorf("token.ExpiresAt = %v, want ~%v (diff %v)", token.ExpiresAt, expiresAt, diff)
	}
	if token.UsedAt != nil {
		t.Errorf("token.UsedAt = %v, want nil (発行直後は未使用)", token.UsedAt)
	}
	if token.CreatedAt.IsZero() {
		t.Error("token.CreatedAt は DB 既定値で設定されるはず")
	}
	if token.UpdatedAt.IsZero() {
		t.Error("token.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestPasswordResetTokenRepository_FindByTokenDigest verifies that
// FindByTokenDigest returns the token whose stored digest matches, and (nil, nil)
// for an unknown digest (a normal lookup outcome, not an error).
//
// [Ja] TestPasswordResetTokenRepository_FindByTokenDigest は、FindByTokenDigest が
// 保存ダイジェストの一致するトークンを返し、未知のダイジェストには (nil, nil) を返す
// (正常なルックアップ結果でありエラーではない) ことを検証する。
func TestPasswordResetTokenRepository_FindByTokenDigest(t *testing.T) {
	t.Parallel()

	repo, tx, ctx := newPasswordResetTokenRepo(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	created, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "lookup-digest",
		ExpiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("ダイジェストの一致するトークンを取得できる", func(t *testing.T) {
		token, err := repo.FindByTokenDigest(ctx, "lookup-digest")
		if err != nil {
			t.Fatalf("FindByTokenDigest() error = %v", err)
		}
		if token == nil {
			t.Fatal("FindByTokenDigest() = nil, want token")
		}
		if token.ID != created.ID {
			t.Errorf("token.ID = %v, want %v", token.ID, created.ID)
		}
		if token.UserID != userID {
			t.Errorf("token.UserID = %v, want %v", token.UserID, userID)
		}
	})

	t.Run("未知のダイジェストは (nil, nil) を返す", func(t *testing.T) {
		token, err := repo.FindByTokenDigest(ctx, "unknown-digest")
		if err != nil {
			t.Fatalf("FindByTokenDigest() error = %v, want nil", err)
		}
		if token != nil {
			t.Errorf("FindByTokenDigest() = %v, want nil", token)
		}
	})
}

// TestPasswordResetTokenRepository_MarkAsUsed verifies that MarkAsUsed stamps
// used_at so a freshly issued (unused) token becomes used, which the model's
// IsUsed then reports.
//
// [Ja] TestPasswordResetTokenRepository_MarkAsUsed は、MarkAsUsed が used_at を打刻し、
// 発行直後の (未使用) トークンが使用済みになる (モデルの IsUsed がそれを報告する) ことを
// 検証する。
func TestPasswordResetTokenRepository_MarkAsUsed(t *testing.T) {
	t.Parallel()

	repo, tx, ctx := newPasswordResetTokenRepo(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	created, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "to-be-used",
		ExpiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.IsUsed() {
		t.Fatal("発行直後のトークンは未使用のはず")
	}

	if err := repo.MarkAsUsed(ctx, created.ID); err != nil {
		t.Fatalf("MarkAsUsed() error = %v", err)
	}

	token, err := repo.FindByTokenDigest(ctx, "to-be-used")
	if err != nil {
		t.Fatalf("FindByTokenDigest() error = %v", err)
	}
	if token == nil {
		t.Fatal("打刻後もトークンは取得できるはず")
	}
	if !token.IsUsed() {
		t.Error("MarkAsUsed() 後のトークンは使用済みのはず")
	}
}

// TestPasswordResetTokenRepository_DeleteUnusedByUserID verifies that
// DeleteUnusedByUserID removes the user's unused tokens but leaves a used
// (spent) one in place, so issuing a new token invalidates outstanding links
// without erasing the record of a past reset.
//
// [Ja] TestPasswordResetTokenRepository_DeleteUnusedByUserID は、
// DeleteUnusedByUserID がユーザーの未使用トークンを削除しつつ、使用済み (消費済み) の
// トークンは残すことを検証する。新しいトークンの発行で未使用リンクを無効化しつつ、過去の
// リセットの記録を消さないためである。
func TestPasswordResetTokenRepository_DeleteUnusedByUserID(t *testing.T) {
	t.Parallel()

	repo, tx, ctx := newPasswordResetTokenRepo(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	expiresAt := time.Now().Add(model.PasswordResetTokenExpirationDuration)

	// Two unused tokens and one already-used token for the same user.
	//
	// [Ja] 同一ユーザーに対し、未使用トークン 2 つと使用済みトークン 1 つ。
	for _, digest := range []string{"unused-1", "unused-2"} {
		if _, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
			UserID:      userID,
			TokenDigest: digest,
			ExpiresAt:   expiresAt,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	usedToken, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "used-1",
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := tx.Exec(ctx, "UPDATE password_reset_tokens SET used_at = NOW() WHERE id = $1", uuid.UUID(usedToken.ID)); err != nil {
		t.Fatalf("使用済みトークンの打刻に失敗: %v", err)
	}

	if got := countTokens(t, tx, ctx, userID); got != 3 {
		t.Fatalf("削除前のトークン数 = %d, want 3", got)
	}

	if err := repo.DeleteUnusedByUserID(ctx, userID); err != nil {
		t.Fatalf("DeleteUnusedByUserID() error = %v", err)
	}

	// Only the used token survives.
	//
	// [Ja] 残るのは使用済みトークンだけ。
	if got := countTokens(t, tx, ctx, userID); got != 1 {
		t.Errorf("削除後のトークン数 = %d, want 1 (使用済みのみ残る)", got)
	}
}
