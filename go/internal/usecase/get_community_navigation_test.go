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

// newGetCommunityNavigationUsecaseはテストが所有するデータベース上にUseCaseを
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

// TestGetCommunityNavigationUsecase_Executeは、Executeがコミュニティと、それが
// 並べた順の掲示板を、カテゴリーで束ねずフラットに返すこと、そしてどのカテゴリーにも
// 属さない掲示板が属する掲示板と並んで返ることを検証します (ADR 0011)。
func TestGetCommunityNavigationUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, categoryRepo, boardRepo, db := newGetCommunityNavigationUsecase(t)
	ctx := context.Background()

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communitiesへのINSERTに失敗: %v", err)
	}

	hobby, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "hobby", Name: "趣味", Position: 2})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	communityCategory, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "community", Name: "コミュニティ", Position: 1})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	createBoard := func(categoryID *model.CategoryID, slug, name string, position int) {
		t.Helper()
		if _, err := boardRepo.Create(ctx, repository.CreateBoardInput{
			CategoryID: categoryID,
			Slug:       slug,
			Name:       name,
			Position:   position,
		}); err != nil {
			t.Fatalf("Create()のエラー = %v", err)
		}
	}

	// 掲示板は期待する並びともその逆とも異なる順で作る。挿入順をそのまま返すだけの
	// 結果では通らないようにするためである。
	createBoard(&hobby.ID, "games", "ゲーム", 3)
	createBoard(&communityCategory.ID, "chat", "雑談", 0)
	createBoard(nil, "questions", "質問", 2)
	createBoard(&communityCategory.ID, "announcements", "お知らせ", 1)

	nav, err := uc.Execute(ctx, usecase.GetCommunityNavigationInput{})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if nav.Community == nil {
		t.Fatal("nav.Community = nil、期待値 = 非nil")
	}
	if nav.Community.Name != "ジャズ喫茶" {
		t.Errorf("nav.Community.Name = %q、期待値 = %q", nav.Community.Name, "ジャズ喫茶")
	}

	wantNames := []string{"雑談", "お知らせ", "質問", "ゲーム"}
	if len(nav.Boards) != len(wantNames) {
		t.Fatalf("len(nav.Boards) = %d、期待値 = %d", len(nav.Boards), len(wantNames))
	}
	for i, want := range wantNames {
		if nav.Boards[i].Name != want {
			t.Errorf("nav.Boards[%d].Name = %q、期待値 = %q", i, nav.Boards[i].Name, want)
		}
	}
	if nav.Boards[2].CategoryID != nil {
		t.Errorf("nav.Boards[2].CategoryID = %v、期待値 = nil (どのカテゴリーにも属さない掲示板)", nav.Boards[2].CategoryID)
	}
}

// TestGetCommunityNavigationUsecase_Execute_EmptyInstanceは、コミュニティも
// 掲示板も持たないデータベースがエラー無しで応答されることを検証します。それは
// マイグレーション直後のインスタンスが置かれている状態であり、サイドバーはそれでも
// 描画されなければならないためです。
func TestGetCommunityNavigationUsecase_Execute_EmptyInstance(t *testing.T) {
	t.Parallel()

	uc, _, _, _ := newGetCommunityNavigationUsecase(t)

	nav, err := uc.Execute(context.Background(), usecase.GetCommunityNavigationInput{})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if nav.Community != nil {
		t.Errorf("nav.Community = %+v、期待値 = nil", nav.Community)
	}
	if len(nav.Boards) != 0 {
		t.Errorf("len(nav.Boards) = %d、期待値 = 0", len(nav.Boards))
	}
}

// TestGetCommunityNavigationUsecase_Execute_CanAccessAdminは、サイドバーの
// 管理画面への導線が何から描かれるかを検証します。管理者は許され、ロールを1つも持たない
// アカウントは許されず、そもそもアカウントを持たない訪問者も許されません。
func TestGetCommunityNavigationUsecase_Execute_CanAccessAdmin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// seedは訪問者のロールをデータベースへ置き、ナビゲーションを読む対象のidを
		// 返す。匿名の訪問者のときはnilを返す。
		seed               func(t *testing.T, db *database.DB) *model.UserID
		wantCanAccessAdmin bool
	}{
		{
			name: "adminロールを持つ利用者には管理画面への導線が出る",
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
				t.Fatalf("Execute()のエラー = %v", err)
			}
			if nav.CanAccessAdmin != tt.wantCanAccessAdmin {
				t.Errorf("nav.CanAccessAdmin = %t、期待値 = %t", nav.CanAccessAdmin, tt.wantCanAccessAdmin)
			}
		})
	}
}

// TestGetCommunityNavigationUsecase_Execute_AnonymousVisitorReadsNoRolesは、
// 匿名の訪問者がロールのためのクエリを1つも払わないことを検証します。コミュニティと
// 掲示板は専用のデータベースから読み、ロールはクローズ済みのデータベースから読むため、
// ロールのために発行されたクエリはすべて失敗します。したがってエラー無しで答えが返るのは、
// 1つも発行されなかった場合だけです。
//
// これをExecuteの読解に委ねず検証するのは、コミュニティの公開ページが匿名の訪問者の
// 最も多く読むページであり、そこにクエリが1つ増えれば、アカウントを持たないすべての人が
// そのすべてのページでそれを払うことになるためです。
func TestGetCommunityNavigationUsecase_Execute_AnonymousVisitorReadsNoRoles(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	closedDB := testutil.SetupDB(t)
	if err := closedDB.Close(); err != nil {
		t.Fatalf("Close()のエラー = %v", err)
	}

	uc := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(db),
		repository.NewBoardRepository(db),
		repository.NewRoleRepository(closedDB),
	)

	if _, err := uc.Execute(context.Background(), usecase.GetCommunityNavigationInput{}); err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil (匿名の訪問者はロールを読まない)", err)
	}

	userID := testutil.NewUserBuilder(t, db).Build()
	if _, err := uc.Execute(context.Background(), usecase.GetCommunityNavigationInput{UserID: &userID}); err == nil {
		t.Error("Execute()のエラー = nil、エラーを期待 (サインイン済みの訪問者はロールを読む)")
	}
}
