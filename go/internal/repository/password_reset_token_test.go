package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newPasswordResetTokenRepoはテストが所有するデータベース上に
// PasswordResetTokenRepositoryを作る。呼び出し側はそのdbを手元に持ち続け、
// (FKの対象となる) ユーザーを仕込んだり行を直接数えたりする。
func newPasswordResetTokenRepo(t *testing.T, db *database.DB) (*repository.PasswordResetTokenRepository, context.Context) {
	t.Helper()
	return repository.NewPasswordResetTokenRepository(db), context.Background()
}

// countTokensはテストのデータベース内で、そのユーザーの
// password_reset_tokens行数を返す。
func countTokens(t *testing.T, db *database.DB, ctx context.Context, userID model.UserID) int {
	t.Helper()
	var count int
	if err := db.Writer.QueryRowContext(ctx, "SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = ?", int64(userID)).Scan(&count); err != nil {
		t.Fatalf("トークン数の取得に失敗: %v", err)
	}
	return count
}

// TestPasswordResetTokenRepository_Createは、Createがトークンを永続化し、DBが
// 採番したidとタイムスタンプ・保存したダイジェストと有効期限・nilのused_at (発行直後の
// トークンは未使用) を伴って返すことを検証する。
func TestPasswordResetTokenRepository_Create(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	repo, ctx := newPasswordResetTokenRepo(t, db)
	userID := testutil.NewUserBuilder(t, db).Build()

	expiresAt := time.Now().Add(model.PasswordResetTokenExpirationDuration)
	token, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "digest-abc",
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if token.ID == 0 {
		t.Error("Create() token.IDはDB採番で空でないはず")
	}
	if token.UserID != userID {
		t.Errorf("token.UserID = %v、期待値 = %v", token.UserID, userID)
	}
	if token.TokenDigest != "digest-abc" {
		t.Errorf("token.TokenDigest = %q、期待値 = %q", token.TokenDigest, "digest-abc")
	}
	// 時刻は小数部3桁固定で保存されるため、往復した値はナノ秒精度の入力と最大
	// 1ミリ秒ずれうる。完全一致ではなくその許容差で比較する。
	if diff := token.ExpiresAt.Sub(expiresAt); diff < -time.Millisecond || diff > time.Millisecond {
		t.Errorf("token.ExpiresAt = %v、期待値 = %v 前後 (差 %v)", token.ExpiresAt, expiresAt, diff)
	}
	if token.UsedAt != nil {
		t.Errorf("token.UsedAt = %v、期待値 = nil (発行直後は未使用)", token.UsedAt)
	}
	if token.CreatedAt.IsZero() {
		t.Error("token.CreatedAtはDB既定値で設定されるはず")
	}
	if token.UpdatedAt.IsZero() {
		t.Error("token.UpdatedAtはDB既定値で設定されるはず")
	}
}

// TestPasswordResetTokenRepository_FindByTokenDigestは、FindByTokenDigestが
// 保存ダイジェストの一致するトークンを返し、未知のダイジェストには (nil, nil) を返す
// (正常なルックアップ結果でありエラーではない) ことを検証する。
func TestPasswordResetTokenRepository_FindByTokenDigest(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	repo, ctx := newPasswordResetTokenRepo(t, db)
	userID := testutil.NewUserBuilder(t, db).Build()
	created, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "lookup-digest",
		ExpiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("ダイジェストの一致するトークンを取得できる", func(t *testing.T) {
		token, err := repo.FindByTokenDigest(ctx, "lookup-digest")
		if err != nil {
			t.Fatalf("FindByTokenDigest()のエラー = %v", err)
		}
		if token == nil {
			t.Fatal("FindByTokenDigest() = nil、期待値はトークン")
		}
		if token.ID != created.ID {
			t.Errorf("token.ID = %v、期待値 = %v", token.ID, created.ID)
		}
		if token.UserID != userID {
			t.Errorf("token.UserID = %v、期待値 = %v", token.UserID, userID)
		}
	})

	t.Run("未知のダイジェストは (nil, nil) を返す", func(t *testing.T) {
		token, err := repo.FindByTokenDigest(ctx, "unknown-digest")
		if err != nil {
			t.Fatalf("FindByTokenDigest()のエラー = %v、期待値 = nil", err)
		}
		if token != nil {
			t.Errorf("FindByTokenDigest() = %v、期待値 = nil", token)
		}
	})
}

// TestPasswordResetTokenRepository_MarkAsUsedは、MarkAsUsedがused_atを打刻し、
// 発行直後の (未使用) トークンが使用済みになる (モデルのIsUsedがそれを報告する) ことを
// 検証する。
func TestPasswordResetTokenRepository_MarkAsUsed(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	repo, ctx := newPasswordResetTokenRepo(t, db)
	userID := testutil.NewUserBuilder(t, db).Build()
	created, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "to-be-used",
		ExpiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if created.IsUsed() {
		t.Fatal("発行直後のトークンは未使用のはず")
	}

	if err := repo.MarkAsUsed(ctx, created.ID); err != nil {
		t.Fatalf("MarkAsUsed()のエラー = %v", err)
	}

	token, err := repo.FindByTokenDigest(ctx, "to-be-used")
	if err != nil {
		t.Fatalf("FindByTokenDigest()のエラー = %v", err)
	}
	if token == nil {
		t.Fatal("打刻後もトークンは取得できるはず")
	}
	if !token.IsUsed() {
		t.Error("MarkAsUsed() 後のトークンは使用済みのはず")
	}
}

// TestPasswordResetTokenRepository_DeleteUnusedByUserIDは、
// DeleteUnusedByUserIDがユーザーの未使用トークンを削除しつつ、使用済み (消費済み) の
// トークンは残すことを検証する。新しいトークンの発行で未使用リンクを無効化しつつ、過去の
// リセットの記録を消さないためである。
func TestPasswordResetTokenRepository_DeleteUnusedByUserID(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	repo, ctx := newPasswordResetTokenRepo(t, db)
	userID := testutil.NewUserBuilder(t, db).Build()
	expiresAt := time.Now().Add(model.PasswordResetTokenExpirationDuration)

	// 同一ユーザーに対し、未使用トークン2つと使用済みトークン1つ。
	for _, digest := range []string{"unused-1", "unused-2"} {
		if _, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
			UserID:      userID,
			TokenDigest: digest,
			ExpiresAt:   expiresAt,
		}); err != nil {
			t.Fatalf("Create()のエラー = %v", err)
		}
	}
	usedToken, err := repo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: "used-1",
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := db.Writer.ExecContext(ctx, "UPDATE password_reset_tokens SET used_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE id = ?", int64(usedToken.ID)); err != nil {
		t.Fatalf("使用済みトークンの打刻に失敗: %v", err)
	}

	if got := countTokens(t, db, ctx, userID); got != 3 {
		t.Fatalf("削除前のトークン数 = %d、期待値 = 3", got)
	}

	if err := repo.DeleteUnusedByUserID(ctx, userID); err != nil {
		t.Fatalf("DeleteUnusedByUserID()のエラー = %v", err)
	}

	// 残るのは使用済みトークンだけ。
	if got := countTokens(t, db, ctx, userID); got != 1 {
		t.Errorf("削除後のトークン数 = %d、期待値 = 1 (使用済みのみ残る)", got)
	}
}
