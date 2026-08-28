package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newCommunityRepo builds a CommunityRepository over a database the test owns,
// returning the database as well so a test can insert the community row itself.
// Nothing in the application creates that row, so the test writes it directly.
//
// [Ja] newCommunityRepo はテストが所有するデータベース上に CommunityRepository を
// 作り、テストがコミュニティの行を自分で挿入できるようデータベースも返します。
// アプリケーションにはこの行を作るものが無いため、テストは直接書き込みます。
func newCommunityRepo(t *testing.T) (*repository.CommunityRepository, *database.DB, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	return repository.NewCommunityRepository(db), db, context.Background()
}

// TestCommunityRepository_Find verifies that Find returns the single community
// row, and that a database without one is reported as absence rather than an
// error — the state every migrated database starts in.
//
// [Ja] TestCommunityRepository_Find は Find が唯一のコミュニティの行を返すこと、
// そして行を持たないデータベースがエラーではなく未存在として報告されることを検証します。
// 後者はマイグレーション済みのデータベースが必ず最初に置かれている状態です。
func TestCommunityRepository_Find(t *testing.T) {
	t.Parallel()

	t.Run("コミュニティを取得できる", func(t *testing.T) {
		t.Parallel()

		repo, db, ctx := newCommunityRepo(t)

		if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
			t.Fatalf("communities への INSERT に失敗: %v", err)
		}

		community, err := repo.Find(ctx)
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if community == nil {
			t.Fatal("Find() = nil, want community")
		}
		if community.ID != 1 {
			t.Errorf("community.ID = %d, want %d", community.ID, 1)
		}
		if community.Name != "ジャズ喫茶" {
			t.Errorf("community.Name = %q, want %q", community.Name, "ジャズ喫茶")
		}
		if community.CreatedAt.IsZero() {
			t.Error("community.CreatedAt は DB 既定値で設定されるはず")
		}
		if community.UpdatedAt.IsZero() {
			t.Error("community.UpdatedAt は DB 既定値で設定されるはず")
		}
	})

	t.Run("行が無いときは nil を返す", func(t *testing.T) {
		t.Parallel()

		repo, _, ctx := newCommunityRepo(t)

		community, err := repo.Find(ctx)
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if community != nil {
			t.Errorf("Find() = %+v, want nil", community)
		}
	})
}
