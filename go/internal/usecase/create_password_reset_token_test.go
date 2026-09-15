package usecase_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

const testAppURL = "https://groobb.example.dev"

// newCreatePasswordResetTokenUsecaseはテスト専用のデータベース上でUseCaseを
// 組み立てる。CreatePasswordResetTokenUsecaseはそのWriterで自前のトランザクションを
// 開く。ユーザーリポジトリ (テストがユーザーを仕込むため) とフェイクのジョブインサーター
// (どのメールが投入されたかをテストが検証するため) を返す。
func newCreatePasswordResetTokenUsecase(t *testing.T, db *database.DB) (*usecase.CreatePasswordResetTokenUsecase, *repository.UserRepository, *testutil.FakeJobInserter) {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)

	inserter := &testutil.FakeJobInserter{}
	cfg := &config.Config{Env: "test", AppURL: testAppURL}

	uc := usecase.NewCreatePasswordResetTokenUsecase(
		db.Writer,
		validator.NewPasswordResetCreateValidator(),
		userRepo,
		passwordResetTokenRepo,
		dispatcher.NewDispatcher(inserter),
		cfg,
	)
	return uc, userRepo, inserter
}

// seedUserはユニークなemailを持つコミット済みユーザーを作成して返す。
func seedUser(t *testing.T, ctx context.Context, db *database.DB, userRepo *repository.UserRepository) *model.User {
	t.Helper()

	email := "pwreset@example.com"
	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	return user
}

// countTokensはそのユーザーのpassword_reset_tokens行数を返す。
func countTokens(t *testing.T, db *database.DB, ctx context.Context, userID model.UserID) int {
	t.Helper()
	var count int
	if err := db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = ?", int64(userID)).Scan(&count); err != nil {
		t.Fatalf("トークン数の取得に失敗: %v", err)
	}
	return count
}

// digestForUserはそのユーザーに保存された唯一のtoken_digestを返す。
func digestForUser(t *testing.T, db *database.DB, ctx context.Context, userID model.UserID) string {
	t.Helper()
	var digest string
	if err := db.Reader.QueryRowContext(ctx, "SELECT token_digest FROM password_reset_tokens WHERE user_id = ?", int64(userID)).Scan(&digest); err != nil {
		t.Fatalf("token_digestの取得に失敗: %v", err)
	}
	return digest
}

// TestCreatePasswordResetTokenUsecase_Execute_Successは、既知のemailがちょうど
// 1つのトークンを発行し、そのアドレスへリセットメールを投入し、投入されたリンクが、その
// ハッシュが保存済みダイジェストと一致するトークンを運ぶこと (リンクと保存行が同じトークンの
// 両端であること) を検証する。
func TestCreatePasswordResetTokenUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, userRepo, inserter := newCreatePasswordResetTokenUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	user := seedUser(t, ctx, db, userRepo)

	out, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  user.Email,
		Locale: "ja",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if out == nil || out.Token == nil {
		t.Fatal("Execute()のoutput / Token = nil、作成されたトークンを期待")
	}

	if got := countTokens(t, db, ctx, user.ID); got != 1 {
		t.Errorf("発行後のトークン数 = %d、期待値 = 1", got)
	}

	if !inserter.Called {
		t.Fatal("リセットメールのジョブが投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendPasswordResetArgs)
	if !ok {
		t.Fatalf("投入ジョブの型 = %T、期待値 = SendPasswordResetArgs", inserter.Args)
	}
	if args.Email != user.Email {
		t.Errorf("args.Email = %q、期待値 = %q", args.Email, user.Email)
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q、期待値 = %q", args.Locale, "ja")
	}

	// リセットURLは設定済みアプリURLに編集パスとtokenクエリパラメータを付けた
	// もので、そのtokenのハッシュが保存済みダイジェストと一致する。
	parsed, err := url.Parse(args.ResetURL)
	if err != nil {
		t.Fatalf("ResetURLのパースに失敗: %v", err)
	}
	if got := testAppURL + "/password/edit"; parsed.Scheme+"://"+parsed.Host+parsed.Path != got {
		t.Errorf("ResetURLのパス = %q、期待値 = %q", parsed.Scheme+"://"+parsed.Host+parsed.Path, got)
	}
	rawToken := parsed.Query().Get("token")
	if rawToken == "" {
		t.Fatal("ResetURLにtokenクエリパラメータが無い")
	}
	if auth.HashToken(rawToken) != digestForUser(t, db, ctx, user.ID) {
		t.Error("リンクのトークンのハッシュが保存済みダイジェストと一致しない")
	}
}

// TestCreatePasswordResetTokenUsecase_Execute_UnknownEmailは列挙攻撃対策の
// no-op経路を検証する。どのアカウントにも一致しないemailはトークンを発行せず、メールも
// 投入せず、(nil, nil) を返すため、呼び出し側は成功経路と区別できない。
func TestCreatePasswordResetTokenUsecase_Execute_UnknownEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, _, inserter := newCreatePasswordResetTokenUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	email := "nobody@example.com"
	out, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  email,
		Locale: "ja",
	})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
	}
	if out != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil (未知のemailでは発行しない)", out)
	}
	if inserter.Called {
		t.Error("未知のemailではメールを投入すべきでない")
	}
}

// TestCreatePasswordResetTokenUsecase_Execute_InvalidEmailは、形式不正のemailが
// ValidationErrorを返し、トークンを発行せず、メールも投入しないことを検証する。
func TestCreatePasswordResetTokenUsecase_Execute_InvalidEmail(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, _, inserter := newCreatePasswordResetTokenUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	out, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  "not-an-email",
		Locale: "ja",
	})
	if out != nil {
		t.Errorf("Execute()のoutput = %v、期待値 = nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute()のエラー = %v、期待値 = *model.ValidationError", err)
	}
	if inserter.Called {
		t.Error("形式不正のemailではメールを投入すべきでない")
	}
}

// TestCreatePasswordResetTokenUsecase_Execute_ReplacesOutstandingTokenは、
// 未使用トークンが既にある状態でリセットを申請するとそれが置き換えられ、ユーザーには
// ちょうど1つの有効なトークンが残る (以前のリンクは無効化される) ことを検証する。
func TestCreatePasswordResetTokenUsecase_Execute_ReplacesOutstandingToken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, userRepo, _ := newCreatePasswordResetTokenUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	user := seedUser(t, ctx, db, userRepo)

	for i := 0; i < 2; i++ {
		if _, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
			Email:  user.Email,
			Locale: "ja",
		}); err != nil {
			t.Fatalf("%d 回目のExecute()のエラー = %v", i+1, err)
		}
	}

	if got := countTokens(t, db, ctx, user.ID); got != 1 {
		t.Errorf("2回申請後のトークン数 = %d、期待値 = 1 (古い未使用トークンは置き換えられる)", got)
	}
}
