package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newUpdatePasswordResetUsecase wires the usecase over the test's own database.
// It returns the repositories so a test can seed a user, its password, and a
// token, then assert the updated password and spent token.
//
// [Ja] newUpdatePasswordResetUsecase はテスト専用のデータベース上に UseCase を組み立てる。
// テストがユーザー・そのパスワード・トークンを仕込み、更新後のパスワードと消費済みトークンを
// 検証できるようリポジトリを返す。
func newUpdatePasswordResetUsecase(t *testing.T, db *database.DB) (*usecase.UpdatePasswordResetUsecase, *repository.UserRepository, *repository.UserPasswordRepository, *repository.PasswordResetTokenRepository) {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)

	uc := usecase.NewUpdatePasswordResetUsecase(
		db.Writer,
		validator.NewPasswordUpdateValidator(passwordResetTokenRepo),
		passwordResetTokenRepo,
		userPasswordRepo,
	)
	return uc, userRepo, userPasswordRepo, passwordResetTokenRepo
}

// seedUserWithPassword creates a committed user with an initial password,
// returning the user id so a test can reset that password through the usecase.
//
// [Ja] seedUserWithPassword は初期パスワードを持つコミット済みユーザーを作成し、テストが
// その UseCase でパスワードをリセットできるようユーザー id を返す。
func seedUserWithPassword(t *testing.T, ctx context.Context, db *database.DB, userRepo *repository.UserRepository, userPasswordRepo *repository.UserPasswordRepository, email, password string) model.UserID {
	t.Helper()

	user, err := userRepo.Create(ctx, repository.CreateUserInput{Email: email, Atname: testutil.UniqueAtname(db), Locale: "ja", TimeZone: "Asia/Tokyo"})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	digest, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{UserID: user.ID, PasswordDigest: digest}); err != nil {
		t.Fatalf("パスワード資格情報の作成に失敗: %v", err)
	}
	return user.ID
}

// TestUpdatePasswordResetUsecase_Execute_Success verifies that a usable token and
// a valid password replace the user's password (the new password verifies, the
// old one no longer does) and spend the token (it becomes used).
//
// [Ja] TestUpdatePasswordResetUsecase_Execute_Success は、使えるトークンと有効な
// パスワードがユーザーのパスワードを置き換え (新しいパスワードで検証でき、古いパスワードでは
// できなくなる)、トークンを消費する (使用済みになる) ことを検証する。
func TestUpdatePasswordResetUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc, userRepo, userPasswordRepo, tokenRepo := newUpdatePasswordResetUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	email := "pw-update-success@example.com"
	userID := seedUserWithPassword(t, ctx, db, userRepo, userPasswordRepo, email, "oldpassword123")

	rawToken := "success-token"
	if _, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: auth.HashToken(rawToken),
		ExpiresAt:   time.Now().Add(model.PasswordResetTokenExpirationDuration),
	}); err != nil {
		t.Fatalf("トークンの作成に失敗: %v", err)
	}

	if err := uc.Execute(ctx, usecase.UpdatePasswordResetInput{
		Token:                rawToken,
		Password:             "newpassword123",
		PasswordConfirmation: "newpassword123",
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// The new password verifies and the old one no longer does.
	//
	// [Ja] 新しいパスワードで検証でき、古いパスワードではもう検証できない。
	password, err := userPasswordRepo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if password == nil {
		t.Fatal("更新後のパスワード資格情報を引けない")
	}
	if err := auth.CheckPassword(password.PasswordDigest, "newpassword123"); err != nil {
		t.Errorf("新しいパスワードで検証できない: %v", err)
	}
	if err := auth.CheckPassword(password.PasswordDigest, "oldpassword123"); err == nil {
		t.Error("古いパスワードはもう検証できないはず")
	}

	// The token is spent.
	//
	// [Ja] トークンは消費済み。
	token, err := tokenRepo.FindByTokenDigest(ctx, auth.HashToken(rawToken))
	if err != nil {
		t.Fatalf("FindByTokenDigest() error = %v", err)
	}
	if token == nil || !token.IsUsed() {
		t.Error("更新後のトークンは使用済みのはず")
	}
}

// TestUpdatePasswordResetUsecase_Execute_RejectsBadInput verifies that an
// unusable token (unknown, used, expired) or an invalid password returns a
// ValidationError and leaves the password unchanged.
//
// [Ja] TestUpdatePasswordResetUsecase_Execute_RejectsBadInput は、使えないトークン
// (未知・使用済み・期限切れ) または不正なパスワードが ValidationError を返し、パスワードを
// 変更しないままにすることを検証する。
func TestUpdatePasswordResetUsecase_Execute_RejectsBadInput(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	tests := []struct {
		name string
		// setupToken returns the raw token string to submit, after seeding any
		// matching row for the user.
		//
		// [Ja] setupToken はユーザー向けの一致行を仕込んだ上で、送信する平文トークン
		// 文字列を返す。
		setupToken func(t *testing.T, ctx context.Context, tokenRepo *repository.PasswordResetTokenRepository, userID model.UserID) string
		password   string
	}{
		{
			name: "未知のトークン",
			setupToken: func(t *testing.T, ctx context.Context, tokenRepo *repository.PasswordResetTokenRepository, userID model.UserID) string {
				return "unknown-token"
			},
			password: "newpassword123",
		},
		{
			name: "使用済みのトークン",
			setupToken: func(t *testing.T, ctx context.Context, tokenRepo *repository.PasswordResetTokenRepository, userID model.UserID) string {
				raw := "used-token"
				token, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{UserID: userID, TokenDigest: auth.HashToken(raw), ExpiresAt: time.Now().Add(model.PasswordResetTokenExpirationDuration)})
				if err != nil {
					t.Fatalf("トークンの作成に失敗: %v", err)
				}
				if err := tokenRepo.MarkAsUsed(ctx, token.ID); err != nil {
					t.Fatalf("トークンの使用済みマークに失敗: %v", err)
				}
				return raw
			},
			password: "newpassword123",
		},
		{
			name: "期限切れのトークン",
			setupToken: func(t *testing.T, ctx context.Context, tokenRepo *repository.PasswordResetTokenRepository, userID model.UserID) string {
				raw := "expired-token"
				if _, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{UserID: userID, TokenDigest: auth.HashToken(raw), ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
					t.Fatalf("トークンの作成に失敗: %v", err)
				}
				return raw
			},
			password: "newpassword123",
		},
		{
			name: "不正なパスワード (短すぎ)",
			setupToken: func(t *testing.T, ctx context.Context, tokenRepo *repository.PasswordResetTokenRepository, userID model.UserID) string {
				raw := "valid-token"
				if _, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{UserID: userID, TokenDigest: auth.HashToken(raw), ExpiresAt: time.Now().Add(model.PasswordResetTokenExpirationDuration)}); err != nil {
					t.Fatalf("トークンの作成に失敗: %v", err)
				}
				return raw
			},
			password: "short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, userRepo, userPasswordRepo, tokenRepo := newUpdatePasswordResetUsecase(t, db)
			ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

			email := testutil.UniqueEmail(db, "pw-update-bad")
			userID := seedUserWithPassword(t, ctx, db, userRepo, userPasswordRepo, email, "oldpassword123")
			rawToken := tt.setupToken(t, ctx, tokenRepo, userID)

			err := uc.Execute(ctx, usecase.UpdatePasswordResetInput{
				Token:                rawToken,
				Password:             tt.password,
				PasswordConfirmation: tt.password,
			})
			if ve := model.AsValidationError(err); ve == nil {
				t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
			}

			// The original password is untouched.
			//
			// [Ja] 元のパスワードは変更されていない。
			password, err := userPasswordRepo.FindByUserID(ctx, userID)
			if err != nil {
				t.Fatalf("FindByUserID() error = %v", err)
			}
			if err := auth.CheckPassword(password.PasswordDigest, "oldpassword123"); err != nil {
				t.Errorf("失敗時は元のパスワードが保たれるはず: %v", err)
			}
		})
	}
}
