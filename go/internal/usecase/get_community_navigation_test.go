package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetCommunityNavigationUsecase builds the UseCase over a database the test
// owns, returning the repositories and the database alongside it so the test can
// arrange the rows it is about to read back.
//
// [Ja] newGetCommunityNavigationUsecase はテストが所有するデータベース上に UseCase を
// 構築し、これから読み戻す行をテストが用意できるよう、リポジトリとデータベースも併せて
// 返します。
func newGetCommunityNavigationUsecase(t *testing.T) (*usecase.GetCommunityNavigationUsecase, *repository.CategoryRepository, *repository.BoardRepository, *database.DB) {
	t.Helper()

	db := testutil.SetupDB(t)
	communityRepo := repository.NewCommunityRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	roleRepo := repository.NewRoleRepository(db)

	return usecase.NewGetCommunityNavigationUsecase(communityRepo, boardRepo, roleRepo), categoryRepo, boardRepo, db
}

// TestGetCommunityNavigationUsecase_Execute verifies that Execute returns the
// community together with its boards in the order the community placed them,
// flat rather than grouped by category, and that a board belonging to no
// category is listed among the ones that do (ADR 0011).
//
// [Ja] TestGetCommunityNavigationUsecase_Execute は、Execute がコミュニティと、それが
// 並べた順の掲示板を、カテゴリーで束ねずフラットに返すこと、そしてどのカテゴリーにも
// 属さない掲示板が属する掲示板と並んで返ることを検証します (ADR 0011)。
func TestGetCommunityNavigationUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, categoryRepo, boardRepo, db := newGetCommunityNavigationUsecase(t)
	ctx := context.Background()

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communities への INSERT に失敗: %v", err)
	}

	hobby, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "hobby", Name: "趣味", Position: 2})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	communityCategory, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "community", Name: "コミュニティ", Position: 1})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	createBoard := func(categoryID *model.CategoryID, slug, name string, position int) {
		t.Helper()
		if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
			CategoryID: categoryID,
			Slug:       slug,
			Name:       name,
			Position:   position,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// The boards are created in neither the expected order nor its reverse, so a
	// result that merely echoes the insertion order cannot pass.
	//
	// [Ja] 掲示板は期待する並びともその逆とも異なる順で作る。挿入順をそのまま返すだけの
	// 結果では通らないようにするためである。
	createBoard(&hobby.ID, "games", "ゲーム", 3)
	createBoard(&communityCategory.ID, "chat", "雑談", 0)
	createBoard(nil, "questions", "質問", 2)
	createBoard(&communityCategory.ID, "announcements", "お知らせ", 1)

	nav, err := uc.Execute(ctx, usecase.GetCommunityNavigationInput{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if nav.Community == nil {
		t.Fatal("nav.Community = nil, want community")
	}
	if nav.Community.Name != "ジャズ喫茶" {
		t.Errorf("nav.Community.Name = %q, want %q", nav.Community.Name, "ジャズ喫茶")
	}

	wantNames := []string{"雑談", "お知らせ", "質問", "ゲーム"}
	if len(nav.Boards) != len(wantNames) {
		t.Fatalf("len(nav.Boards) = %d, want %d", len(nav.Boards), len(wantNames))
	}
	for i, want := range wantNames {
		if nav.Boards[i].Name != want {
			t.Errorf("nav.Boards[%d].Name = %q, want %q", i, nav.Boards[i].Name, want)
		}
	}
	if nav.Boards[2].CategoryID != nil {
		t.Errorf("nav.Boards[2].CategoryID = %v, want nil (どのカテゴリーにも属さない掲示板)", nav.Boards[2].CategoryID)
	}
}

// TestGetCommunityNavigationUsecase_Execute_EmptyInstance verifies that a
// database holding neither a community nor a board is answered without an
// error, since that is the state a freshly migrated instance is in and the
// sidebar still has to render.
//
// [Ja] TestGetCommunityNavigationUsecase_Execute_EmptyInstance は、コミュニティも
// 掲示板も持たないデータベースがエラー無しで応答されることを検証します。それは
// マイグレーション直後のインスタンスが置かれている状態であり、サイドバーはそれでも
// 描画されなければならないためです。
func TestGetCommunityNavigationUsecase_Execute_EmptyInstance(t *testing.T) {
	t.Parallel()

	uc, _, _, _ := newGetCommunityNavigationUsecase(t)

	nav, err := uc.Execute(context.Background(), usecase.GetCommunityNavigationInput{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if nav.Community != nil {
		t.Errorf("nav.Community = %+v, want nil", nav.Community)
	}
	if len(nav.Boards) != 0 {
		t.Errorf("len(nav.Boards) = %d, want 0", len(nav.Boards))
	}
}

// TestGetCommunityNavigationUsecase_Execute_CanAccessAdmin verifies what the
// sidebar's link into the administration screens is drawn from: an administrator
// is admitted, an account holding no role is not, and neither is a visitor with
// no account at all.
//
// [Ja] TestGetCommunityNavigationUsecase_Execute_CanAccessAdmin は、サイドバーの
// 管理画面への導線が何から描かれるかを検証します。管理者は許され、ロールを 1 つも持たない
// アカウントは許されず、そもそもアカウントを持たない訪問者も許されません。
func TestGetCommunityNavigationUsecase_Execute_CanAccessAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// seed places the visitor's roles in the database and returns the id the
		// navigation is read for, or nil for an anonymous visitor.
		//
		// [Ja] seed は訪問者のロールをデータベースへ置き、ナビゲーションを読む対象の id を
		// 返す。匿名の訪問者のときは nil を返す。
		seed               func(t *testing.T, db *database.DB) *model.UserID
		wantCanAccessAdmin bool
	}{
		{
			name: "admin ロールを持つ利用者には管理画面への導線が出る",
			seed: func(t *testing.T, db *database.DB) *model.UserID {
				t.Helper()
				userID := testutil.NewUserBuilder(t, db).Build()
				testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
				return &userID
			},
			wantCanAccessAdmin: true,
		},
		{
			name: "ロールを持たない利用者には管理画面への導線が出ない",
			seed: func(t *testing.T, db *database.DB) *model.UserID {
				t.Helper()
				userID := testutil.NewUserBuilder(t, db).Build()
				return &userID
			},
		},
		{
			name: "匿名の訪問者には管理画面への導線が出ない",
			seed: func(t *testing.T, db *database.DB) *model.UserID {
				t.Helper()
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, _, _, db := newGetCommunityNavigationUsecase(t)

			nav, err := uc.Execute(context.Background(), usecase.GetCommunityNavigationInput{
				UserID: tt.seed(t, db),
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if nav.CanAccessAdmin != tt.wantCanAccessAdmin {
				t.Errorf("nav.CanAccessAdmin = %t, want %t", nav.CanAccessAdmin, tt.wantCanAccessAdmin)
			}
		})
	}
}

// TestGetCommunityNavigationUsecase_Execute_AnonymousVisitorReadsNoRoles
// verifies that an anonymous visitor costs no query for roles. The community and
// the boards are read from a database of their own, while the roles would be read
// from one that has been closed, so any query issued for them fails. An answer
// without an error is therefore only possible if none was issued.
//
// This is asserted rather than left to the reading of Execute because the
// community's public pages are what an anonymous visitor reads most, and a query
// added there would be paid on every one of them by everyone holding no account.
//
// [Ja] TestGetCommunityNavigationUsecase_Execute_AnonymousVisitorReadsNoRoles は、
// 匿名の訪問者がロールのためのクエリを 1 つも払わないことを検証します。コミュニティと
// 掲示板は専用のデータベースから読み、ロールはクローズ済みのデータベースから読むため、
// ロールのために発行されたクエリはすべて失敗します。したがってエラー無しで答えが返るのは、
// 1 つも発行されなかった場合だけです。
//
// これを Execute の読解に委ねず検証するのは、コミュニティの公開ページが匿名の訪問者の
// 最も多く読むページであり、そこにクエリが 1 つ増えれば、アカウントを持たないすべての人が
// そのすべてのページでそれを払うことになるためです。
func TestGetCommunityNavigationUsecase_Execute_AnonymousVisitorReadsNoRoles(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	closedDB := testutil.SetupDB(t)
	if err := closedDB.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	uc := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(db),
		repository.NewBoardRepository(db),
		repository.NewRoleRepository(closedDB),
	)

	if _, err := uc.Execute(context.Background(), usecase.GetCommunityNavigationInput{}); err != nil {
		t.Fatalf("Execute() error = %v, want no error (匿名の訪問者はロールを読まない)", err)
	}

	userID := testutil.NewUserBuilder(t, db).Build()
	if _, err := uc.Execute(context.Background(), usecase.GetCommunityNavigationInput{UserID: &userID}); err == nil {
		t.Error("Execute() error = nil, want error (サインイン済みの訪問者はロールを読む)")
	}
}
