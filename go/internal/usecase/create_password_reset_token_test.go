package usecase_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

const testAppURL = "https://groobb.example.dev"

// newCreatePasswordResetTokenUsecase wires the usecase over the shared pool (not
// a rolled-back transaction) because CreatePasswordResetTokenUsecase opens its
// own transaction internally; an outer transaction's seed rows would be invisible
// to that inner transaction. It returns the user repository (so a test can seed a
// user and the test commits with unique emails) and the fake job inserter (so a
// test can assert what mail was enqueued).
//
// [Ja] newCreatePasswordResetTokenUsecase は共有プール (ロールバックされる
// トランザクションではなく) で UseCase を組み立てる。CreatePasswordResetTokenUsecase は
// 内部で自前のトランザクションを開くため、外側トランザクションで仕込んだ行はその内側
// トランザクションから見えないからである。ユーザーリポジトリ (テストがユーザーを仕込み、
// ユニークな email でコミットするため) とフェイクのジョブインサーター (どのメールが投入
// されたかをテストが検証するため) を返す。
func newCreatePasswordResetTokenUsecase(t *testing.T) (*usecase.CreatePasswordResetTokenUsecase, *repository.UserRepository, *testutil.FakeJobInserter) {
	t.Helper()

	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(queries)

	inserter := &testutil.FakeJobInserter{}
	cfg := &config.Config{Env: "test", AppURL: testAppURL}

	uc := usecase.NewCreatePasswordResetTokenUsecase(
		db,
		validator.NewPasswordResetCreateValidator(),
		userRepo,
		passwordResetTokenRepo,
		dispatcher.NewDispatcher(inserter),
		cfg,
	)
	return uc, userRepo, inserter
}

// seedUser creates a committed user with a unique email and returns it.
//
// [Ja] seedUser はユニークな email を持つコミット済みユーザーを作成して返す。
func seedUser(t *testing.T, ctx context.Context, userRepo *repository.UserRepository) *model.User {
	t.Helper()

	email := fmt.Sprintf("pwreset-%s@example.com", uuid.NewString())
	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(),
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	return user
}

// countTokens returns how many password_reset_tokens rows exist for the user.
//
// [Ja] countTokens はそのユーザーの password_reset_tokens 行数を返す。
func countTokens(t *testing.T, ctx context.Context, userID model.UserID) int {
	t.Helper()
	var count int
	if err := testutil.GetTestDB().QueryRow(ctx, "SELECT COUNT(*) FROM password_reset_tokens WHERE user_id = $1", uuid.UUID(userID)).Scan(&count); err != nil {
		t.Fatalf("トークン数の取得に失敗: %v", err)
	}
	return count
}

// digestForUser returns the single stored token_digest for the user.
//
// [Ja] digestForUser はそのユーザーに保存された唯一の token_digest を返す。
func digestForUser(t *testing.T, ctx context.Context, userID model.UserID) string {
	t.Helper()
	var digest string
	if err := testutil.GetTestDB().QueryRow(ctx, "SELECT token_digest FROM password_reset_tokens WHERE user_id = $1", uuid.UUID(userID)).Scan(&digest); err != nil {
		t.Fatalf("token_digest の取得に失敗: %v", err)
	}
	return digest
}

// TestCreatePasswordResetTokenUsecase_Execute_Success verifies that a known email
// issues exactly one token, enqueues the reset mail to that address, and that the
// enqueued link carries a token whose hash equals the stored digest (so the link
// and the stored row are the two ends of the same token).
//
// [Ja] TestCreatePasswordResetTokenUsecase_Execute_Success は、既知の email がちょうど
// 1 つのトークンを発行し、そのアドレスへリセットメールを投入し、投入されたリンクが、その
// ハッシュが保存済みダイジェストと一致するトークンを運ぶこと (リンクと保存行が同じトークンの
// 両端であること) を検証する。
func TestCreatePasswordResetTokenUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	uc, userRepo, inserter := newCreatePasswordResetTokenUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	user := seedUser(t, ctx, userRepo)

	out, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  user.Email,
		Locale: "ja",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.Token == nil {
		t.Fatal("Execute() output / Token = nil, want a created token")
	}

	if got := countTokens(t, ctx, user.ID); got != 1 {
		t.Errorf("発行後のトークン数 = %d, want 1", got)
	}

	if !inserter.Called {
		t.Fatal("リセットメールのジョブが投入されていない")
	}
	args, ok := inserter.Args.(dispatcher.SendPasswordResetArgs)
	if !ok {
		t.Fatalf("投入ジョブの型 = %T, want SendPasswordResetArgs", inserter.Args)
	}
	if args.Email != user.Email {
		t.Errorf("args.Email = %q, want %q", args.Email, user.Email)
	}
	if args.Locale != "ja" {
		t.Errorf("args.Locale = %q, want %q", args.Locale, "ja")
	}

	// The reset URL is the configured app URL plus the edit path and a token query
	// param, and that token hashes to the stored digest.
	//
	// [Ja] リセット URL は設定済みアプリ URL に編集パスと token クエリパラメータを付けた
	// もので、その token のハッシュが保存済みダイジェストと一致する。
	parsed, err := url.Parse(args.ResetURL)
	if err != nil {
		t.Fatalf("ResetURL のパースに失敗: %v", err)
	}
	if got := testAppURL + "/password/edit"; parsed.Scheme+"://"+parsed.Host+parsed.Path != got {
		t.Errorf("ResetURL のパス = %q, want %q", parsed.Scheme+"://"+parsed.Host+parsed.Path, got)
	}
	rawToken := parsed.Query().Get("token")
	if rawToken == "" {
		t.Fatal("ResetURL に token クエリパラメータが無い")
	}
	if auth.HashToken(rawToken) != digestForUser(t, ctx, user.ID) {
		t.Error("リンクのトークンのハッシュが保存済みダイジェストと一致しない")
	}
}

// TestCreatePasswordResetTokenUsecase_Execute_UnknownEmail verifies the
// enumeration-safe no-op path: an email that matches no account issues no token,
// enqueues no mail, and returns (nil, nil) so the caller cannot tell it apart
// from the success path.
//
// [Ja] TestCreatePasswordResetTokenUsecase_Execute_UnknownEmail は列挙攻撃対策の
// no-op 経路を検証する。どのアカウントにも一致しない email はトークンを発行せず、メールも
// 投入せず、(nil, nil) を返すため、呼び出し側は成功経路と区別できない。
func TestCreatePasswordResetTokenUsecase_Execute_UnknownEmail(t *testing.T) {
	t.Parallel()

	uc, _, inserter := newCreatePasswordResetTokenUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	email := fmt.Sprintf("nobody-%s@example.com", uuid.NewString())
	out, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  email,
		Locale: "ja",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if out != nil {
		t.Errorf("Execute() output = %v, want nil (未知の email では発行しない)", out)
	}
	if inserter.Called {
		t.Error("未知の email ではメールを投入すべきでない")
	}
}

// TestCreatePasswordResetTokenUsecase_Execute_InvalidEmail verifies that a
// malformed email returns a ValidationError, issues no token, and enqueues no
// mail.
//
// [Ja] TestCreatePasswordResetTokenUsecase_Execute_InvalidEmail は、形式不正の email が
// ValidationError を返し、トークンを発行せず、メールも投入しないことを検証する。
func TestCreatePasswordResetTokenUsecase_Execute_InvalidEmail(t *testing.T) {
	t.Parallel()

	uc, _, inserter := newCreatePasswordResetTokenUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	out, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
		Email:  "not-an-email",
		Locale: "ja",
	})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if inserter.Called {
		t.Error("形式不正の email ではメールを投入すべきでない")
	}
}

// TestCreatePasswordResetTokenUsecase_Execute_ReplacesOutstandingToken verifies
// that requesting a reset while an unused token already exists replaces it, so
// the user is left with exactly one live token (the earlier link is invalidated).
//
// [Ja] TestCreatePasswordResetTokenUsecase_Execute_ReplacesOutstandingToken は、
// 未使用トークンが既にある状態でリセットを申請するとそれが置き換えられ、ユーザーには
// ちょうど 1 つの有効なトークンが残る (以前のリンクは無効化される) ことを検証する。
func TestCreatePasswordResetTokenUsecase_Execute_ReplacesOutstandingToken(t *testing.T) {
	t.Parallel()

	uc, userRepo, _ := newCreatePasswordResetTokenUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	user := seedUser(t, ctx, userRepo)

	for i := 0; i < 2; i++ {
		if _, err := uc.Execute(ctx, usecase.CreatePasswordResetTokenInput{
			Email:  user.Email,
			Locale: "ja",
		}); err != nil {
			t.Fatalf("Execute() #%d error = %v", i+1, err)
		}
	}

	if got := countTokens(t, ctx, user.ID); got != 1 {
		t.Errorf("2 回申請後のトークン数 = %d, want 1 (古い未使用トークンは置き換えられる)", got)
	}
}
