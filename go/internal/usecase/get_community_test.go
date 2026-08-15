package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetCommunityUsecase wires the read UseCase over the test transaction, which
// it can do (unlike the creating UseCase) because reading opens no transaction of
// its own, so the seeded rows and the lookup share one and roll back together. It
// returns the repository the test seeds through and a context carrying the locale
// the not-found message is resolved in.
//
// [Ja] newGetCommunityUsecase は読み取り UseCase をテスト用トランザクションで組み立てる。
// (作成の UseCase と違い) 読み取りは自前のトランザクションを開かないためこれができ、仕込んだ
// 行とルックアップが 1 つのトランザクションを共有して一緒にロールバックされる。テストが行を
// 仕込むリポジトリと、未存在メッセージを解決するロケールを載せた context も返す。
func newGetCommunityUsecase(t *testing.T) (*usecase.GetCommunityUsecase, *repository.CommunityRepository, context.Context) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	communityRepo := repository.NewCommunityRepository(query.New(db)).WithTx(tx)
	return usecase.NewGetCommunityUsecase(communityRepo), communityRepo, i18n.SetLocale(context.Background(), i18n.LangJa)
}

// TestGetCommunityUsecase_Execute_Success verifies that an identifier that names
// a community resolves it.
//
// [Ja] TestGetCommunityUsecase_Execute_Success は、コミュニティを指す識別子がそれを
// 解決することを検証する。
func TestGetCommunityUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	uc, communityRepo, ctx := newGetCommunityUsecase(t)

	created, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "取得テストコミュニティ",
		Identifier: "get-community",
	})
	if err != nil {
		t.Fatalf("コミュニティの作成に失敗: %v", err)
	}

	out, err := uc.Execute(ctx, usecase.GetCommunityInput{Identifier: "get-community"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.Community == nil {
		t.Fatal("Execute() はコミュニティを返すはず")
	}
	if out.Community.ID != created.ID {
		t.Errorf("community.ID = %s, want %s", out.Community.ID, created.ID)
	}
	if out.Community.Name != "取得テストコミュニティ" {
		t.Errorf("community.Name = %q, want %q", out.Community.Name, "取得テストコミュニティ")
	}
}

// TestGetCommunityUsecase_Execute_NotFound verifies that an identifier nobody has
// claimed comes back as an AppError carrying the not-found code and a localized
// message, which is what lets the handler answer 404 rather than treating the
// absence as a system error.
//
// [Ja] TestGetCommunityUsecase_Execute_NotFound は、誰も取得していない識別子が未存在の
// コードとローカライズ済みメッセージを持つ AppError として返ることを検証する。これにより
// ハンドラーは、未存在をシステムエラーとして扱わず 404 で応答できる。
func TestGetCommunityUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	uc, _, ctx := newGetCommunityUsecase(t)

	out, err := uc.Execute(ctx, usecase.GetCommunityInput{Identifier: "no-such-community"})
	if out != nil {
		t.Error("未存在の識別子で output が返っている")
	}

	ae := model.AsAppError(err)
	if ae == nil {
		t.Fatalf("Execute() error = %v, want *model.AppError", err)
	}
	if ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("ae.Code = %d, want %d (AppErrCodeResourceNotFound)", ae.Code, model.AppErrCodeResourceNotFound)
	}
	if want := i18n.T(ctx, "error_not_found_message"); ae.UserMsg != want {
		t.Errorf("ae.UserMsg = %q, want %q", ae.UserMsg, want)
	}
}
