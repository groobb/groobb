package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newCommunityRepo builds a CommunityRepository bound to the test transaction,
// exercising WithTx so writes roll back when the test finishes.
//
// [Ja] newCommunityRepo はテスト用トランザクションに束ねた CommunityRepository を作る。
// WithTx を通すことで、テスト終了時に書き込みがロールバックされる。
func newCommunityRepo(t *testing.T) (*repository.CommunityRepository, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	return repository.NewCommunityRepository(query.New(db)).WithTx(tx), context.Background()
}

func TestCommunityRepository_Create(t *testing.T) {
	t.Parallel()

	repo, ctx := newCommunityRepo(t)

	community, err := repo.Create(ctx, repository.CreateCommunityInput{
		Name:       "作成テストコミュニティ",
		Identifier: "create-community",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if community.ID == (model.CommunityID{}) {
		t.Error("Create() community.ID は DB 採番で空でないはず")
	}
	if community.Name != "作成テストコミュニティ" {
		t.Errorf("community.Name = %q, want %q", community.Name, "作成テストコミュニティ")
	}
	if community.Identifier != "create-community" {
		t.Errorf("community.Identifier = %q, want %q", community.Identifier, "create-community")
	}
	if community.CreatedAt.IsZero() {
		t.Error("community.CreatedAt は DB 既定値で設定されるはず")
	}
	if community.UpdatedAt.IsZero() {
		t.Error("community.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestCommunityRepository_FindByIdentifier verifies lookup by identifier: an
// existing identifier resolves the community, the match is case-insensitive via
// citext, and an unknown identifier returns (nil, nil).
//
// [Ja] TestCommunityRepository_FindByIdentifier は identifier による取得を検証する。
// 存在する identifier はコミュニティを解決し、照合は citext により大文字小文字を無視し、
// 未知の identifier は (nil, nil) を返す。
func TestCommunityRepository_FindByIdentifier(t *testing.T) {
	t.Parallel()

	repo, ctx := newCommunityRepo(t)

	created, err := repo.Create(ctx, repository.CreateCommunityInput{
		Name:       "取得テストコミュニティ",
		Identifier: "find-community",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("identifier でコミュニティを取得できる", func(t *testing.T) {
		community, err := repo.FindByIdentifier(ctx, "find-community")
		if err != nil {
			t.Fatalf("FindByIdentifier() error = %v", err)
		}
		if community == nil {
			t.Fatal("FindByIdentifier() = nil, want community")
		}
		if community.ID != created.ID {
			t.Errorf("community.ID = %v, want %v", community.ID, created.ID)
		}
		if community.Name != "取得テストコミュニティ" {
			t.Errorf("community.Name = %q, want %q", community.Name, "取得テストコミュニティ")
		}
	})

	t.Run("citext により大文字小文字を無視して取得できる", func(t *testing.T) {
		community, err := repo.FindByIdentifier(ctx, "Find-Community")
		if err != nil {
			t.Fatalf("FindByIdentifier() error = %v", err)
		}
		if community == nil {
			t.Fatal("FindByIdentifier() = nil, want community")
		}
		if community.ID != created.ID {
			t.Errorf("community.ID = %v, want %v", community.ID, created.ID)
		}
	})

	t.Run("未知の identifier は (nil, nil) を返す", func(t *testing.T) {
		community, err := repo.FindByIdentifier(ctx, "unknown-community")
		if err != nil {
			t.Fatalf("FindByIdentifier() error = %v, want nil", err)
		}
		if community != nil {
			t.Errorf("FindByIdentifier() = %v, want nil", community)
		}
	})
}

// TestCommunityRepository_CreateRejectsDuplicateIdentifier verifies the
// communities.identifier UNIQUE constraint on the citext column rejects a second
// community whose identifier differs only in letter case.
//
// [Ja] TestCommunityRepository_CreateRejectsDuplicateIdentifier は、citext 列である
// communities.identifier の UNIQUE 制約が、大文字小文字だけが異なる identifier を持つ
// 2 つ目のコミュニティを拒否することを確認する。
func TestCommunityRepository_CreateRejectsDuplicateIdentifier(t *testing.T) {
	t.Parallel()

	repo, ctx := newCommunityRepo(t)

	if _, err := repo.Create(ctx, repository.CreateCommunityInput{
		Name:       "1 つ目のコミュニティ",
		Identifier: "duplicate-community",
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	_, err := repo.Create(ctx, repository.CreateCommunityInput{
		Name:       "2 つ目のコミュニティ",
		Identifier: "Duplicate-Community",
	})
	if err == nil {
		t.Error("大文字小文字だけが異なる identifier の Create() はエラーになるはず")
	}
}
