package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newVerifyEmailConfirmationUsecaseはテスト専用のデータベース上でUseCaseを
// 組み立てる。VerifyEmailConfirmationUsecaseはコードの照合と結果の書き込みのために、
// そのWriterで自前のトランザクションを開く。テストが確認を仕込み検証後の状態を確認
// できるようリポジトリも返す。
func newVerifyEmailConfirmationUsecase(t *testing.T, db *database.DB) (*usecase.VerifyEmailConfirmationUsecase, *repository.EmailConfirmationRepository) {
	t.Helper()

	repo := repository.NewEmailConfirmationRepository(db)
	uc := usecase.NewVerifyEmailConfirmationUsecase(
		db.Writer,
		validator.NewEmailConfirmationCreateValidator(repo),
		repo,
	)
	return uc, repo
}

// seedActiveConfirmationは指定コードのコミット済み・アクティブなサインアップ確認を
// 作成し、テストが検証を駆動できるよう返す。
func seedActiveConfirmation(t *testing.T, ctx context.Context, repo *repository.EmailConfirmationRepository, code string) *model.EmailConfirmation {
	t.Helper()

	email := "verify@example.com"
	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: email,
		Event: model.EmailConfirmationEventSignUp,
		Code:  code,
	})
	if err != nil {
		t.Fatalf("確認の作成に失敗: %v", err)
	}
	return confirmation
}

// TestVerifyEmailConfirmationUsecase_Execute_Successは、正しいコードが確認を返し、
// 成功済みとして打刻する (これ以降activeでなくなる) ことを検証する。
func TestVerifyEmailConfirmationUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	out, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "123456"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil || out.EmailConfirmation == nil {
		t.Fatal("Execute()のoutput / EmailConfirmation = nil")
	}
	if out.EmailConfirmation.ID != confirmation.ID {
		t.Errorf("out.EmailConfirmation.ID = %v、期待値 = %v", out.EmailConfirmation.ID, confirmation.ID)
	}

	// 確認は成功済みとして打刻されたため、もはやactiveとして該当しない。
	active, err := repo.FindActiveByID(ctx, confirmation.ID)
	if err != nil {
		t.Fatalf("FindActiveByID()のエラー = %v", err)
	}
	if active != nil {
		t.Error("検証成功後はsucceeded_atが打刻されactiveでなくなるはず")
	}
}

// TestVerifyEmailConfirmationUsecase_Execute_WrongCodeは、誤ったコードがフォーム
// 全体のValidationErrorで失敗し、失敗試行回数をインクリメントし、確認を打刻せず (上限
// 未満ならactiveのまま) 残すことを検証する。これによりユーザーは再試行できる。
func TestVerifyEmailConfirmationUsecase_Execute_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	out, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "000000"})
	if out != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
	}

	// 検証失敗時は確認を打刻してはならず、activeのまま残り、失敗試行回数は1に
	// インクリメントされる。
	active, err := repo.FindActiveByID(ctx, confirmation.ID)
	if err != nil {
		t.Fatalf("FindActiveByID()のエラー = %v", err)
	}
	if active == nil {
		t.Fatal("検証失敗時はsucceeded_atを打刻せずactiveのままのはず")
	}
	if active.FailedAttemptsCount != 1 {
		t.Errorf("active.FailedAttemptsCount = %d、期待値 = 1", active.FailedAttemptsCount)
	}
}

// TestVerifyEmailConfirmationUsecase_Execute_WrongCodeReachesLimitは、誤ったコード
// のインクリメントが上限に達すると確認を無効化することを検証する。上限の1つ手前にある
// 確認は、もう1回の誤った試行でactiveでなくなる。
func TestVerifyEmailConfirmationUsecase_Execute_WrongCodeReachesLimit(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	// カウントを上限 (5) の1つ手前まで進め、次の誤った試行で上限に達するようにする。
	for i := 0; i < 4; i++ {
		if err := repo.IncrementFailedAttempts(ctx, confirmation.ID); err != nil {
			t.Fatalf("IncrementFailedAttempts()のエラー = %v", err)
		}
	}

	_, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "000000"})
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}

	// カウントは上限に達したため、確認はもうactiveでない。
	active, err := repo.FindActiveByID(ctx, confirmation.ID)
	if err != nil {
		t.Fatalf("FindActiveByID()のエラー = %v", err)
	}
	if active != nil {
		t.Error("上限に達した後はactiveでなくなるはず")
	}
}

// TestVerifyEmailConfirmationUsecase_Execute_AttemptsExhaustedは、失敗試行が上限に
// 達すると、正しいコードでさえフォーム全体のValidationErrorで拒否され、確認が成功済みに
// 打刻されないことを検証する (総当たりロックアウトが効いている)。
func TestVerifyEmailConfirmationUsecase_Execute_AttemptsExhausted(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	// 試行回数を使い切る (カウントを上限の5まで進める)。
	for i := 0; i < 5; i++ {
		if err := repo.IncrementFailedAttempts(ctx, confirmation.ID); err != nil {
			t.Fatalf("IncrementFailedAttempts()のエラー = %v", err)
		}
	}

	// 正しいコードでも今は拒否される。
	out, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "123456"})
	if out != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}

	// 確認は成功済みに打刻されていてはならない。
	var succeededAt *time.Time
	if err := db.Reader.QueryRowContext(ctx, `SELECT succeeded_at FROM email_confirmations WHERE id = ?`, int64(confirmation.ID)).Scan(&succeededAt); err != nil {
		t.Fatalf("succeeded_atの読み戻しに失敗: %v", err)
	}
	if succeededAt != nil {
		t.Error("試行回数を使い切った確認は正しいコードでも成功済みにならないはず")
	}
}
