package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newCategoryRepoはテストが所有するデータベース上にCategoryRepositoryを作る。
// リポジトリだけが必要なテストがデータベース自体を抱えずに済むようにするためである。
func newCategoryRepo(t *testing.T) (*repository.CategoryRepository, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	return repository.NewCategoryRepository(db), context.Background()
}

func TestCategoryRepository_Create(t *testing.T) {
	t.Parallel()

	repo, ctx := newCategoryRepo(t)

	category, err := repo.Create(ctx, repository.CreateCategoryInput{
		Slug:     "announcements",
		Name:     "お知らせ",
		Position: 3,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if category.ID == 0 {
		t.Error("Create() category.IDはDB採番で空でないはず")
	}
	if category.Slug != "announcements" {
		t.Errorf("category.Slug = %q、期待値 = %q", category.Slug, "announcements")
	}
	if category.Name != "お知らせ" {
		t.Errorf("category.Name = %q、期待値 = %q", category.Name, "お知らせ")
	}
	if category.Position != 3 {
		t.Errorf("category.Position = %d、期待値 = %d", category.Position, 3)
	}
	if category.CreatedAt.IsZero() {
		t.Error("category.CreatedAtはDB既定値で設定されるはず")
	}
	if category.UpdatedAt.IsZero() {
		t.Error("category.UpdatedAtはDB既定値で設定されるはず")
	}
}

// TestCategoryRepository_Create_RejectsInvalidSlugは、アドレスの規則が受理しない
// slugが保存されずに拒否されることを検証する。列はNOCASE照合であり1つのカテゴリーを
// 2通りの綴りで持つことはできないが、その唯一の綴りを小文字には保たない。そして
// /c/{slug} は大文字小文字の異なるリクエストを、保存されている綴りへリダイレクトする。
// 大文字を含むslugを保存すると、大文字のURLのほうが正規になってしまう。
func TestCategoryRepository_Create_RejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		slug string
	}{
		{name: "大文字を含む", slug: "Announcements"},
		{name: "パスの区切りを含む", slug: "hobby/games"},
		{name: "空文字", slug: ""},
		{name: "最大長超過", slug: strings.Repeat("a", model.SlugMaxLength+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, ctx := newCategoryRepo(t)

			category, err := repo.Create(ctx, repository.CreateCategoryInput{Slug: tt.slug, Name: "お知らせ"})
			if err == nil {
				t.Fatalf("Create()のエラー = nil、期待値はエラー (slug=%q)", tt.slug)
			}
			if category != nil {
				t.Errorf("Create() category = %+v、期待値 = nil", category)
			}

			stored, err := repo.FindBySlug(ctx, tt.slug)
			if err != nil {
				t.Fatalf("FindBySlug()のエラー = %v", err)
			}
			if stored != nil {
				t.Errorf("FindBySlug() = %+v、期待値 = nil (拒否されたslugが保存されている)", stored)
			}
		})
	}
}

func TestCategoryRepository_FindByID(t *testing.T) {
	t.Parallel()

	repo, ctx := newCategoryRepo(t)

	created, err := repo.Create(ctx, repository.CreateCategoryInput{
		Slug:     "music",
		Name:     "音楽",
		Position: 1,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("idでカテゴリーを取得できる", func(t *testing.T) {
		category, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v", err)
		}
		if category == nil {
			t.Fatal("FindByID() = nil、期待値はカテゴリー")
		}
		if category.Slug != "music" {
			t.Errorf("category.Slug = %q、期待値 = %q", category.Slug, "music")
		}
		if category.Name != "音楽" {
			t.Errorf("category.Name = %q、期待値 = %q", category.Name, "音楽")
		}
	})

	t.Run("存在しないidは (nil, nil) を返す", func(t *testing.T) {
		category, err := repo.FindByID(ctx, created.ID+1)
		if err != nil {
			t.Fatalf("FindByID()のエラー = %v、期待値 = nil", err)
		}
		if category != nil {
			t.Errorf("FindByID() = %v、期待値 = nil", category)
		}
	})
}

func TestCategoryRepository_FindBySlug(t *testing.T) {
	t.Parallel()

	repo, ctx := newCategoryRepo(t)

	created, err := repo.Create(ctx, repository.CreateCategoryInput{
		Slug:     "general",
		Name:     "雑談",
		Position: 1,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("slugでカテゴリーを取得できる", func(t *testing.T) {
		category, err := repo.FindBySlug(ctx, "general")
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v", err)
		}
		if category == nil {
			t.Fatal("FindBySlug() = nil、期待値はカテゴリー")
		}
		if category.ID != created.ID {
			t.Errorf("category.ID = %v、期待値 = %v", category.ID, created.ID)
		}
		if category.Name != "雑談" {
			t.Errorf("category.Name = %q、期待値 = %q", category.Name, "雑談")
		}
	})

	t.Run("大文字小文字が違うslugでも同じカテゴリーを取得できる", func(t *testing.T) {
		category, err := repo.FindBySlug(ctx, "GENERAL")
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v", err)
		}
		if category == nil {
			t.Fatal("FindBySlug() = nil、期待値はカテゴリー")
		}
		if category.ID != created.ID {
			t.Errorf("category.ID = %v、期待値 = %v", category.ID, created.ID)
		}
	})

	t.Run("存在しないslugは (nil, nil) を返す", func(t *testing.T) {
		category, err := repo.FindBySlug(ctx, "unknown")
		if err != nil {
			t.Fatalf("FindBySlug()のエラー = %v、期待値 = nil", err)
		}
		if category != nil {
			t.Errorf("FindBySlug() = %v、期待値 = nil", category)
		}
	})
}

// createCategoryは指定したslugとpositionのカテゴリーを挿入し、エラー時は
// テストを失敗させる。nameをslugから導くのは、どの検証もそれに依存しないためで、
// テストのフィクスチャをそのテストが実際に問うているものだけに保つ。
func createCategory(t *testing.T, ctx context.Context, repo *repository.CategoryRepository, slug string, position int) *model.Category {
	t.Helper()

	category, err := repo.Create(ctx, repository.CreateCategoryInput{
		Slug:     slug,
		Name:     "カテゴリー " + slug,
		Position: position,
	})
	if err != nil {
		t.Fatalf("テスト用カテゴリーの作成に失敗: %v", err)
	}

	return category
}
