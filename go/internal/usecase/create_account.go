package usecase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// defaultUserTimeZone is the time zone assigned to a new account. Groobb has no
// timezone source yet (no browser-based detection or settings screen), so a new
// user gets this account-level default; a later task can let the user change it.
// It matches the project's Japanese-first default (users.locale defaults to ja).
//
// [Ja] defaultUserTimeZone は新規アカウントに割り当てるタイムゾーンです。Groobb には
// まだタイムゾーンの供給元 (ブラウザ検出や設定画面) が無いため、新規ユーザーはこの
// アカウントレベルの既定値を持ちます (後続タスクでユーザーが変更できるようにできる)。
// プロジェクトの日本語優先の既定 (users.locale の既定が ja) に揃えています。
const defaultUserTimeZone = "Asia/Tokyo"

// CreateAccountUsecase orchestrates account creation: it reads the email from a
// verified (succeeded) email confirmation, validates the chosen password, and
// creates the user and its password credential in one transaction. The
// confirmation proves the user controls the email, so the email is taken from it
// rather than from the form. Issuing the session (signing the user in) is a
// separate step the handler runs with CreateSessionUsecase.
//
// [Ja] CreateAccountUsecase はアカウント作成を統括します。検証済み (成功済み) のメール
// 確認から email を読み、選んだパスワードを検証し、ユーザーとそのパスワード資格情報を
// 1 トランザクションで作成します。確認はユーザーが email を管理していることを証明する
// ため、email はフォームではなく確認から取ります。セッションの発行 (サインイン) は
// ハンドラーが CreateSessionUsecase で行う別ステップです。
type CreateAccountUsecase struct {
	db                    *pgxpool.Pool
	accountValidator      *validator.AccountCreateValidator
	emailConfirmationRepo *repository.EmailConfirmationRepository
	userRepo              *repository.UserRepository
	userPasswordRepo      *repository.UserPasswordRepository
}

// NewCreateAccountUsecase builds a CreateAccountUsecase from the pool, the
// validator, and the repositories it persists through.
//
// [Ja] NewCreateAccountUsecase はプール・validator・永続化に使うリポジトリから
// CreateAccountUsecase を構築します。
func NewCreateAccountUsecase(
	db *pgxpool.Pool,
	accountValidator *validator.AccountCreateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *CreateAccountUsecase {
	return &CreateAccountUsecase{
		db:                    db,
		accountValidator:      accountValidator,
		emailConfirmationRepo: emailConfirmationRepo,
		userRepo:              userRepo,
		userPasswordRepo:      userPasswordRepo,
	}
}

// CreateAccountInput is the input to Execute. EmailConfirmationID is the verified
// confirmation's id (carried from the handoff cookie) whose email becomes the new
// user's; Password / PasswordConfirmation are the chosen credential; Locale is
// the request locale stored as the account default.
//
// [Ja] CreateAccountInput は Execute の入力です。EmailConfirmationID は検証済みの確認の
// id (受け渡し Cookie から運ばれる) で、その email が新規ユーザーの email になります。
// Password / PasswordConfirmation は選んだ資格情報、Locale はアカウント既定として保存
// するリクエストのロケールです。
type CreateAccountInput struct {
	EmailConfirmationID  model.EmailConfirmationID
	Password             string
	PasswordConfirmation string
	Locale               string
}

// CreateAccountOutput carries the created user so the handler can issue a session
// for it (sign the new user in).
//
// [Ja] CreateAccountOutput は作成されたユーザーを運び、ハンドラーがそのユーザーの
// セッションを発行 (新規ユーザーをサインイン) できるようにします。
type CreateAccountOutput struct {
	User *model.User
}

// Execute resolves the verified confirmation, validates the password, hashes it,
// and creates the account. The confirmation is read first: without a usable
// verified confirmation the flow is broken, so it returns an AppError (the
// handler restarts sign-up) before any password validation. Password hashing
// runs before the transaction so bcrypt's cost is not paid while holding a row
// lock.
//
// [Ja] Execute は検証済みの確認を解決し、パスワードを検証し、ハッシュ化して、アカウントを
// 作成します。確認を先に読みます。使える検証済み確認が無ければフローは破綻しているため、
// パスワード検証より前に AppError を返します (ハンドラーがサインアップをやり直させる)。
// パスワードのハッシュ化は、行ロックを保持したまま bcrypt のコストを払わないよう、
// トランザクションの前に実行します。
func (uc *CreateAccountUsecase) Execute(ctx context.Context, input CreateAccountInput) (*CreateAccountOutput, error) {
	confirmation, err := uc.emailConfirmationRepo.FindSucceededByID(ctx, input.EmailConfirmationID)
	if err != nil {
		return nil, fmt.Errorf("検証済みメール確認の取得に失敗: %w", err)
	}
	if confirmation == nil {
		// No usable verified confirmation: the handoff is stale, already used, or
		// the code was never verified. This is a known business-level failure, not
		// a user-fixable form error, so return an AppError; the handler sends the
		// user back to start sign-up over rather than re-rendering the form.
		//
		// [Ja] 使える検証済み確認が無い: 受け渡しが失効・使用済み、またはコードが未検証。
		// これはユーザーが修正できるフォームエラーではなく業務レベルの既知の失敗のため
		// AppError を返す。ハンドラーはフォームを再描画する代わりにユーザーをサインアップの
		// やり直しへ送る。
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "validation_code_incorrect_or_expired"),
			Internal: fmt.Errorf("成功済みのメール確認が見つからない: id=%s", input.EmailConfirmationID),
			Metadata: map[string]string{"email_confirmation_id": input.EmailConfirmationID.String()},
		}
	}

	if err := uc.accountValidator.Validate(ctx, validator.AccountCreateValidatorInput{
		Password:             input.Password,
		PasswordConfirmation: input.PasswordConfirmation,
	}); err != nil {
		return nil, err
	}

	passwordDigest, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("パスワードのハッシュ化に失敗: %w", err)
	}

	return uc.createAccount(ctx, confirmation.Email, input.Locale, passwordDigest)
}

// createAccount creates the user and its password credential in one transaction,
// so an account never exists without its password (or vice versa). The
// password digest is computed by Execute beforehand, keeping the transaction to
// pure persistence.
//
// [Ja] createAccount はユーザーとそのパスワード資格情報を 1 トランザクションで作成し、
// パスワードの無いアカウント (またはその逆) が決して生じないようにします。パスワード
// ダイジェストは事前に Execute が計算済みで、トランザクションを純粋な永続化に保ちます。
func (uc *CreateAccountUsecase) createAccount(ctx context.Context, email, locale, passwordDigest string) (*CreateAccountOutput, error) {
	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userRepo := uc.userRepo.WithTx(tx)
	userPasswordRepo := uc.userPasswordRepo.WithTx(tx)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
		Locale:   locale,
		TimeZone: defaultUserTimeZone,
	})
	if err != nil {
		return nil, fmt.Errorf("ユーザーの作成に失敗: %w", err)
	}

	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: passwordDigest,
	}); err != nil {
		return nil, fmt.Errorf("パスワード資格情報の作成に失敗: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateAccountOutput{User: user}, nil
}
