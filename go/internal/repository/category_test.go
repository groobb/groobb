package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newCategoryRepo builds a CategoryRepository over a database the test owns, so
// a test that only needs the repository does not have to hold on to the database
// itself.
//
// [Ja] newCategoryRepo はテストが所有するデータベース上に CategoryRepository を作る。
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
		t.Fatalf("Create() error = %v", err)
	}

	if category.ID == 0 {
		t.Error("Create() category.ID は DB 採番で空でないはず")
	}
	if category.Slug != "announcements" {
		t.Errorf("category.Slug = %q, want %q", category.Slug, "announcements")
	}
	if category.Name != "お知らせ" {
		t.Errorf("category.Name = %q, want %q", category.Name, "お知らせ")
	}
	if category.Position != 3 {
		t.Errorf("category.Position = %d, want %d", category.Position, 3)
	}
	if category.CreatedAt.IsZero() {
		t.Error("category.CreatedAt は DB 既定値で設定されるはず")
	}
	if category.UpdatedAt.IsZero() {
		t.Error("category.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestCategoryRepository_Create_RejectsInvalidSlug verifies that a slug the
// address rule does not accept is refused instead of stored. The column collates
// NOCASE and so cannot hold two spellings of one category, but it does not keep
// the one spelling lowercase, and /c/{slug} redirects a differently cased request
// to whatever is stored. Storing an uppercase slug would make the uppercase URL
// the canonical one.
//
// [Ja] TestCategoryRepository_Create_RejectsInvalidSlug は、アドレスの規則が受理しない
// slug が保存されずに拒否されることを検証する。列は NOCASE 照合であり 1 つのカテゴリーを
// 2 通りの綴りで持つことはできないが、その唯一の綴りを小文字には保たない。そして
// /c/{slug} は大文字小文字の異なるリクエストを、保存されている綴りへリダイレクトする。
// 大文字を含む slug を保存すると、大文字の URL のほうが正規になってしまう。
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
				t.Fatalf("Create() error = nil, want error (slug=%q)", tt.slug)
			}
			if category != nil {
				t.Errorf("Create() category = %+v, want nil", category)
			}

			stored, err := repo.FindBySlug(ctx, tt.slug)
			if err != nil {
				t.Fatalf("FindBySlug() error = %v", err)
			}
			if stored != nil {
				t.Errorf("FindBySlug() = %+v, want nil (拒否された slug が保存されている)", stored)
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
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("id でカテゴリーを取得できる", func(t *testing.T) {
		category, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v", err)
		}
		if category == nil {
			t.Fatal("FindByID() = nil, want category")
		}
		if category.Slug != "music" {
			t.Errorf("category.Slug = %q, want %q", category.Slug, "music")
		}
		if category.Name != "音楽" {
			t.Errorf("category.Name = %q, want %q", category.Name, "音楽")
		}
	})

	t.Run("存在しない id は (nil, nil) を返す", func(t *testing.T) {
		category, err := repo.FindByID(ctx, created.ID+1)
		if err != nil {
			t.Fatalf("FindByID() error = %v, want nil", err)
		}
		if category != nil {
			t.Errorf("FindByID() = %v, want nil", category)
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
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("slug でカテゴリーを取得できる", func(t *testing.T) {
		category, err := repo.FindBySlug(ctx, "general")
		if err != nil {
			t.Fatalf("FindBySlug() error = %v", err)
		}
		if category == nil {
			t.Fatal("FindBySlug() = nil, want category")
		}
		if category.ID != created.ID {
			t.Errorf("category.ID = %v, want %v", category.ID, created.ID)
		}
		if category.Name != "雑談" {
			t.Errorf("category.Name = %q, want %q", category.Name, "雑談")
		}
	})

	t.Run("大文字小文字が違う slug でも同じカテゴリーを取得できる", func(t *testing.T) {
		category, err := repo.FindBySlug(ctx, "GENERAL")
		if err != nil {
			t.Fatalf("FindBySlug() error = %v", err)
		}
		if category == nil {
			t.Fatal("FindBySlug() = nil, want category")
		}
		if category.ID != created.ID {
			t.Errorf("category.ID = %v, want %v", category.ID, created.ID)
		}
	})

	t.Run("存在しない slug は (nil, nil) を返す", func(t *testing.T) {
		category, err := repo.FindBySlug(ctx, "unknown")
		if err != nil {
			t.Fatalf("FindBySlug() error = %v, want nil", err)
		}
		if category != nil {
			t.Errorf("FindBySlug() = %v, want nil", category)
		}
	})
}

// createCategory inserts a category with the given slug and position, failing
// the test on error. Name is derived from the slug because no assertion depends
// on it, which keeps a test's fixtures down to what it is actually about.
//
// [Ja] createCategory は指定した slug と position のカテゴリーを挿入し、エラー時は
// テストを失敗させる。name を slug から導くのは、どの検証もそれに依存しないためで、
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
