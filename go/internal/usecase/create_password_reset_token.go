package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreatePasswordResetTokenUsecaseはパスワードリセット申請を統括します。emailを
// 検証し、使い捨てのリセットトークンを発行し (ハッシュのみ保存)、リセットリンクを運ぶ
// メールを投入します。列挙攻撃に対して安全です。未知のemailではトークンもメールも作らず、
// 呼び出し側への結果は同じになるため、レスポンスはアカウントの存在有無を決して明かしません。
// トークン自体は別のパスワード更新フローで消費されます。
type CreatePasswordResetTokenUsecase struct {
	writer                 *sql.DB
	passwordResetValidator *validator.PasswordResetCreateValidator
	userRepo               *repository.UserRepository
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
	dispatcher             *dispatcher.Dispatcher
	cfg                    *config.Config
}

// NewCreatePasswordResetTokenUsecaseは書き込み用プール・validator・リポジトリ・dispatcher・
// configからCreatePasswordResetTokenUsecaseを構築します。
func NewCreatePasswordResetTokenUsecase(
	writer *sql.DB,
	passwordResetValidator *validator.PasswordResetCreateValidator,
	userRepo *repository.UserRepository,
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	dispatcher *dispatcher.Dispatcher,
	cfg *config.Config,
) *CreatePasswordResetTokenUsecase {
	return &CreatePasswordResetTokenUsecase{
		writer:                 writer,
		passwordResetValidator: passwordResetValidator,
		userRepo:               userRepo,
		passwordResetTokenRepo: passwordResetTokenRepo,
		dispatcher:             dispatcher,
		cfg:                    cfg,
	}
}

// CreatePasswordResetTokenInputはExecuteの入力です。Localeはリクエストの
// ロケールで、リセットメールをユーザーが閲覧中の言語で描画するために運びます。
type CreatePasswordResetTokenInput struct {
	Email  string
	Locale model.Locale
}

// CreatePasswordResetTokenOutputは作成されたリセットトークンを運びます。emailが
// どのアカウントにも一致しなかったとき (列挙攻撃対策のno-op経路) はnilです。ハンドラーは
// 値を無視しどちらでも同じ確認を表示します。出力は呼び出し側やテストが作成経路とno-opを
// 区別できるように存在します。
type CreatePasswordResetTokenOutput struct {
	Token *model.PasswordResetToken
}

// Executeはemailを検証し、アカウントを解決し、存在するときに限り新しいリセット
// トークンを発行してメールを投入します。トークンの生成とハッシュ化はトランザクションの前に
// 実行し、トランザクションは純粋な永続化に保ちます。メール投入の失敗はログに記録しますが
// 返しません。トークンは既に有効で、ここで失敗するとユーザーを手詰まりにし、かつ (エラー
// レスポンスを通じて) そのアドレスがアカウントに属することを明かす恐れがあるためです。
func (uc *CreatePasswordResetTokenUsecase) Execute(ctx context.Context, input CreatePasswordResetTokenInput) (*CreatePasswordResetTokenOutput, error) {
	if err := uc.passwordResetValidator.Validate(ctx, validator.PasswordResetCreateValidatorInput{
		Email: input.Email,
	}); err != nil {
		return nil, err
	}

	user, err := uc.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		// 未知のemail: 何も発行せず、nilのトークンで成功を報告する。これにより
		// ハンドラーは実在アカウントと同じ確認を表示し、レスポンスはそのアドレスが登録
		// 済みかどうかを明かさない。
		return nil, nil
	}

	// 使い捨てトークンとそのダイジェストをトランザクションの前に生成する (永続化では
	// なくロジック)。平文はリセットリンクに入れ、保存するのはダイジェストだけ。
	rawToken, err := auth.GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("リセットトークンの生成に失敗: %w", err)
	}
	tokenDigest := auth.HashToken(rawToken)
	expiresAt := time.Now().Add(model.PasswordResetTokenExpirationDuration)

	token, err := uc.createToken(ctx, user.ID, tokenDigest, expiresAt)
	if err != nil {
		return nil, err
	}

	// リセットリンクを組み立ててメールを投入する。投入の失敗はログに記録して握り潰す。
	// トークンはいずれにせよ有効で、実在アカウントの経路でだけエラーを返すとアカウントの
	// 存在が漏れるため。
	resetURL := fmt.Sprintf("%s/password/edit?token=%s", uc.cfg.AppURL, rawToken)
	if err := uc.dispatcher.EnqueuePasswordReset(ctx, user.Email, resetURL, input.Locale); err != nil {
		slog.ErrorContext(ctx, "パスワードリセットメールのジョブ投入に失敗", "error", err, "user_id", user.ID.String())
	}

	return &CreatePasswordResetTokenOutput{Token: token}, nil
}

// createTokenはユーザーの未使用の既存トークンを、新しく発行した1つに1つの
// トランザクションで置き換えます。新しい申請で以前のリンクを無効化し、ユーザーが複数の
// 有効なリセットトークンを溜め込まないようにします。ダイジェストと有効期限は事前に
// Executeが計算済みで、トランザクションを純粋な永続化に保ちます。
func (uc *CreatePasswordResetTokenUsecase) createToken(ctx context.Context, userID model.UserID, tokenDigest string, expiresAt time.Time) (*model.PasswordResetToken, error) {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	tokenRepo := uc.passwordResetTokenRepo.WithTx(tx)

	if err := tokenRepo.DeleteUnusedByUserID(ctx, userID); err != nil {
		return nil, fmt.Errorf("既存リセットトークンの削除に失敗: %w", err)
	}

	token, err := tokenRepo.Create(ctx, repository.CreatePasswordResetTokenInput{
		UserID:      userID,
		TokenDigest: tokenDigest,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("リセットトークンの作成に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return token, nil
}
