package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// defaultUserTimeZoneは新規アカウントに割り当てるタイムゾーンです。Groobbには
// まだタイムゾーンの供給元 (ブラウザ検出や設定画面) が無いため、新規ユーザーはこの
// アカウントレベルの既定値を持ちます (後続タスクでユーザーが変更できるようにできる)。
// プロジェクトの日本語優先の既定 (users.localeの既定がja) に揃えています。
const defaultUserTimeZone = "Asia/Tokyo"

// CreateAccountUsecaseはアカウント作成を統括します。検証済み (成功済み) のメール
// 確認からemailを読み、選んだパスワードを検証し、ユーザーとそのパスワード資格情報を
// 1トランザクションで作成します。確認はユーザーがemailを管理していることを証明する
// ため、emailはフォームではなく確認から取ります。セッションの発行 (サインイン) は
// ハンドラーがCreateSessionUsecaseで行う別ステップです。
type CreateAccountUsecase struct {
	writer                *sql.DB
	accountValidator      *validator.AccountCreateValidator
	emailConfirmationRepo *repository.EmailConfirmationRepository
	userRepo              *repository.UserRepository
	userPasswordRepo      *repository.UserPasswordRepository
}

// NewCreateAccountUsecaseは書き込み用プール・validator・永続化に使うリポジトリから
// CreateAccountUsecaseを構築します。
func NewCreateAccountUsecase(
	writer *sql.DB,
	accountValidator *validator.AccountCreateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *CreateAccountUsecase {
	return &CreateAccountUsecase{
		writer:                writer,
		accountValidator:      accountValidator,
		emailConfirmationRepo: emailConfirmationRepo,
		userRepo:              userRepo,
		userPasswordRepo:      userPasswordRepo,
	}
}

// CreateAccountInputはExecuteの入力です。EmailConfirmationIDは検証済みの確認の
// id (受け渡しCookieから運ばれる) で、そのemailが新規ユーザーのemailになります。
// Atnameは選んだ @ハンドル、Password / PasswordConfirmationは選んだ資格情報、Localeは
// アカウント既定として保存するリクエストのロケールです。
type CreateAccountInput struct {
	EmailConfirmationID  model.EmailConfirmationID
	Atname               string
	Password             string
	PasswordConfirmation string
	Locale               model.Locale
}

// CreateAccountOutputは作成されたユーザーを運び、ハンドラーがそのユーザーの
// セッションを発行 (新規ユーザーをサインイン) できるようにします。
type CreateAccountOutput struct {
	User *model.User
}

// Executeは検証済みの確認を解決し、パスワードを検証し、ハッシュ化して、アカウントを
// 作成します。確認を先に読みます。使える検証済み確認が無ければフローは破綻しているため、
// パスワード検証より前にAppErrorを返します (ハンドラーがサインアップをやり直させる)。
// パスワードのハッシュ化は、SQLiteの書き込みロックを保持したままbcryptのコストを払わないよう、
// トランザクションの前に実行します。
func (uc *CreateAccountUsecase) Execute(ctx context.Context, input CreateAccountInput) (*CreateAccountOutput, error) {
	confirmation, err := uc.emailConfirmationRepo.FindSucceededByID(ctx, input.EmailConfirmationID)
	if err != nil {
		return nil, fmt.Errorf("検証済みメール確認の取得に失敗: %w", err)
	}
	if confirmation == nil {
		// 使える検証済み確認が無い: 受け渡しが失効・使用済み、またはコードが未検証。
		// これはユーザーが修正できるフォームエラーではなく業務レベルの既知の失敗のため
		// AppErrorを返す。ハンドラーはフォームを再描画する代わりにユーザーをサインアップの
		// やり直しへ送る。
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "validation_code_incorrect_or_expired"),
			Internal: fmt.Errorf("成功済みのメール確認が見つからない: id=%s", input.EmailConfirmationID),
			Metadata: map[string]string{"email_confirmation_id": input.EmailConfirmationID.String()},
		}
	}

	if err := uc.accountValidator.Validate(ctx, validator.AccountCreateValidatorInput{
		Atname:               input.Atname,
		Password:             input.Password,
		PasswordConfirmation: input.PasswordConfirmation,
	}); err != nil {
		return nil, err
	}

	passwordDigest, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("パスワードのハッシュ化に失敗: %w", err)
	}

	return uc.createAccount(ctx, confirmation.Email, input.Atname, input.Locale, passwordDigest)
}

// createAccountはユーザーとそのパスワード資格情報を1トランザクションで作成し、
// パスワードの無いアカウント (またはその逆) が決して生じないようにします。パスワード
// ダイジェストは事前にExecuteが計算済みで、トランザクションを純粋な永続化に保ちます。
func (uc *CreateAccountUsecase) createAccount(ctx context.Context, email, atname string, locale model.Locale, passwordDigest string) (*CreateAccountOutput, error) {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userRepo := uc.userRepo.WithTx(tx)
	userPasswordRepo := uc.userPasswordRepo.WithTx(tx)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   atname,
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

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateAccountOutput{User: user}, nil
}
