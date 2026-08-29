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

// newVerifyEmailConfirmationUsecase wires the usecase over the test's own
// database, whose writer VerifyEmailConfirmationUsecase uses to open its own
// transaction for comparing the code and writing the outcome. It returns the
// repository so a test can seed confirmations and assert the post-verification
// state.
//
// [Ja] newVerifyEmailConfirmationUsecase はテスト専用のデータベース上で UseCase を
// 組み立てる。VerifyEmailConfirmationUsecase はコードの照合と結果の書き込みのために、
// その Writer で自前のトランザクションを開く。テストが確認を仕込み検証後の状態を確認
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

// seedActiveConfirmation creates a committed, active sign-up confirmation with
// the given code and returns it so a test can drive verification.
//
// [Ja] seedActiveConfirmation は指定コードのコミット済み・アクティブなサインアップ確認を
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

// TestVerifyEmailConfirmationUsecase_Execute_Success verifies that a correct code
// returns the confirmation and stamps it as succeeded (so it is no longer
// active).
//
// [Ja] TestVerifyEmailConfirmationUsecase_Execute_Success は、正しいコードが確認を返し、
// 成功済みとして打刻する (これ以降 active でなくなる) ことを検証する。
func TestVerifyEmailConfirmationUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	out, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "123456"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.EmailConfirmation == nil {
		t.Fatal("Execute() output / EmailConfirmation = nil")
	}
	if out.EmailConfirmation.ID != confirmation.ID {
		t.Errorf("out.EmailConfirmation.ID = %v, want %v", out.EmailConfirmation.ID, confirmation.ID)
	}

	// The confirmation is stamped succeeded, so it no longer qualifies as active.
	//
	// [Ja] 確認は成功済みとして打刻されたため、もはや active として該当しない。
	active, err := repo.FindActiveByID(ctx, confirmation.ID)
	if err != nil {
		t.Fatalf("FindActiveByID() error = %v", err)
	}
	if active != nil {
		t.Error("検証成功後は succeeded_at が打刻され active でなくなるはず")
	}
}

// TestVerifyEmailConfirmationUsecase_Execute_WrongCode verifies that a wrong code
// fails with a form-wide ValidationError, increments the failed-attempt count,
// and leaves the confirmation unstamped (still active under the limit) so the
// user can retry.
//
// [Ja] TestVerifyEmailConfirmationUsecase_Execute_WrongCode は、誤ったコードがフォーム
// 全体の ValidationError で失敗し、失敗試行回数をインクリメントし、確認を打刻せず (上限
// 未満なら active のまま) 残すことを検証する。これによりユーザーは再試行できる。
func TestVerifyEmailConfirmationUsecase_Execute_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	out, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "000000"})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasGlobalError() {
		t.Errorf("フォーム全体のエラーが無い: %+v", ve.Global)
	}

	// A failed verification must not stamp the confirmation; it stays active and
	// its failed-attempt count is incremented to 1.
	//
	// [Ja] 検証失敗時は確認を打刻してはならず、active のまま残り、失敗試行回数は 1 に
	// インクリメントされる。
	active, err := repo.FindActiveByID(ctx, confirmation.ID)
	if err != nil {
		t.Fatalf("FindActiveByID() error = %v", err)
	}
	if active == nil {
		t.Fatal("検証失敗時は succeeded_at を打刻せず active のままのはず")
	}
	if active.FailedAttemptsCount != 1 {
		t.Errorf("active.FailedAttemptsCount = %d, want 1", active.FailedAttemptsCount)
	}
}

// TestVerifyEmailConfirmationUsecase_Execute_WrongCodeReachesLimit verifies that
// the wrong-code increment, once it reaches the limit, deactivates the
// confirmation: a confirmation already at one below the limit becomes inactive
// after one more wrong attempt.
//
// [Ja] TestVerifyEmailConfirmationUsecase_Execute_WrongCodeReachesLimit は、誤ったコード
// のインクリメントが上限に達すると確認を無効化することを検証する。上限の 1 つ手前にある
// 確認は、もう 1 回の誤った試行で active でなくなる。
func TestVerifyEmailConfirmationUsecase_Execute_WrongCodeReachesLimit(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	// Drive the count to one below the limit (5), so the next wrong attempt
	// reaches it.
	//
	// [Ja] カウントを上限 (5) の 1 つ手前まで進め、次の誤った試行で上限に達するようにする。
	for i := 0; i < 4; i++ {
		if err := repo.IncrementFailedAttempts(ctx, confirmation.ID); err != nil {
			t.Fatalf("IncrementFailedAttempts() error = %v", err)
		}
	}

	_, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "000000"})
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}

	// The count is now at the limit, so the confirmation is no longer active.
	//
	// [Ja] カウントは上限に達したため、確認はもう active でない。
	active, err := repo.FindActiveByID(ctx, confirmation.ID)
	if err != nil {
		t.Fatalf("FindActiveByID() error = %v", err)
	}
	if active != nil {
		t.Error("上限に達した後は active でなくなるはず")
	}
}

// TestVerifyEmailConfirmationUsecase_Execute_AttemptsExhausted verifies that once
// the failed-attempt limit is reached, even the correct code is rejected with a
// form-wide ValidationError and the confirmation is never stamped succeeded — the
// brute-force lockout holds.
//
// [Ja] TestVerifyEmailConfirmationUsecase_Execute_AttemptsExhausted は、失敗試行が上限に
// 達すると、正しいコードでさえフォーム全体の ValidationError で拒否され、確認が成功済みに
// 打刻されないことを検証する (総当たりロックアウトが効いている)。
func TestVerifyEmailConfirmationUsecase_Execute_AttemptsExhausted(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	uc, repo := newVerifyEmailConfirmationUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	confirmation := seedActiveConfirmation(t, ctx, repo, "123456")

	// Exhaust the attempts (drive the count to the limit of 5).
	//
	// [Ja] 試行回数を使い切る (カウントを上限の 5 まで進める)。
	for i := 0; i < 5; i++ {
		if err := repo.IncrementFailedAttempts(ctx, confirmation.ID); err != nil {
			t.Fatalf("IncrementFailedAttempts() error = %v", err)
		}
	}

	// Even the correct code is now rejected.
	//
	// [Ja] 正しいコードでも今は拒否される。
	out, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: confirmation.ID, Code: "123456"})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}

	// The confirmation must not have been stamped succeeded.
	//
	// [Ja] 確認は成功済みに打刻されていてはならない。
	var succeededAt *time.Time
	if err := db.Reader.QueryRowContext(ctx, `SELECT succeeded_at FROM email_confirmations WHERE id = ?`, int64(confirmation.ID)).Scan(&succeededAt); err != nil {
		t.Fatalf("succeeded_at の読み戻しに失敗: %v", err)
	}
	if succeededAt != nil {
		t.Error("試行回数を使い切った確認は正しいコードでも成功済みにならないはず")
	}
}
