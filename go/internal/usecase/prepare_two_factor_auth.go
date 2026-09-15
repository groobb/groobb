package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// PrepareTwoFactorAuthUsecaseは2段階認証の設定ステップを準備し、QRコードとして
// 表示する登録用secretを解決します。ユーザーに登録中 (未有効化) の設定があれば再利用し、
// なければ新しいsecretを生成して未有効化の行を永続化します。既存secretの再利用により、
// ユーザーが既にスキャンしたQRが再訪しても有効なままになります。行をアクティブな資格情報に
// するのは有効化 (後続ステップ) です。入力はサインイン済みユーザーidだけのためvalidatorは
// 取らず、永続化呼び出しは高々1回のためトランザクションも取りません。
type PrepareTwoFactorAuthUsecase struct {
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewPrepareTwoFactorAuthUsecaseは2FAリポジトリからPrepareTwoFactorAuthUsecaseを
// 構築します。
func NewPrepareTwoFactorAuthUsecase(userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *PrepareTwoFactorAuthUsecase {
	return &PrepareTwoFactorAuthUsecase{userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// PrepareTwoFactorAuthInputはExecuteの入力です。UserIDは2FAを設定するサインイン
// 済みユーザー (セッションで確定する) です。
type PrepareTwoFactorAuthInput struct {
	UserID model.UserID
}

// PrepareTwoFactorAuthOutputはQRコードと手動入力キーとして描画する登録用secretを
// 運びます。AlreadyEnabledはユーザーの2FAが既にアクティブなときtrueで、その場合Secretは
// 空であり、呼び出し側は再登録すべきではありません (設定フォームを表示せず設定ハブへ
// リダイレクトします)。
type PrepareTwoFactorAuthOutput struct {
	Secret         string
	AlreadyEnabled bool
}

// Executeは登録用secretを解決します。ユーザーが既にアクティブな2FAを持つときは
// AlreadyEnabledを報告し、登録中の設定があればそのsecretを再利用し、なければsecretを
// 生成して未有効化の行を挿入します。挿入はON CONFLICT (user_id) DO NOTHINGのため、同時の
// 初回リクエストが先に登録を挿入した場合はCreateがnilを返すので、unique制約で失敗せず
// その行を再解決して再利用します。生成してから挿入する処理は軽い前処理1つを伴う単一の
// 永続化呼び出しのため、Execute内に置き、トランザクションは不要です。
func (uc *PrepareTwoFactorAuthUsecase) Execute(ctx context.Context, input PrepareTwoFactorAuthInput) (*PrepareTwoFactorAuthOutput, error) {
	out, err := uc.resolveExisting(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return out, nil
	}

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		return nil, fmt.Errorf("TOTP secretの生成に失敗: %w", err)
	}

	created, err := uc.userTwoFactorAuthRepo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: input.UserID,
		Secret: secret,
	})
	if err != nil {
		return nil, fmt.Errorf("2段階認証設定の作成に失敗: %w", err)
	}
	if created == nil {
		// 同時のリクエストが先に登録を挿入した (ON CONFLICT DO NOTHINGが行を
		// 返さなかった)。その行を再解決してsecretを再利用し、2つのリクエストが同じ
		// 登録に収束するようにする。
		out, err := uc.resolveExisting(ctx, input.UserID)
		if err != nil {
			return nil, err
		}
		if out == nil {
			return nil, fmt.Errorf("2段階認証設定の作成が競合したが再取得で見つからない: user_id=%s", input.UserID)
		}
		return out, nil
	}

	return &PrepareTwoFactorAuthOutput{Secret: created.Secret}, nil
}

// resolveExistingは既に存在する設定に対する出力を返します。2FAが有効なら
// AlreadyEnabledを、設定がまだ登録中なら再利用可能な登録用secretを返します。ユーザーに
// 設定がまだ無いときは (nil, nil) を返し、呼び出し側に作成を促します。Executeは最初と、
// 挿入競合の後の2回これを呼ぶため、1箇所にまとめることで有効/登録中の分岐の重複を
// 避けます。
func (uc *PrepareTwoFactorAuthUsecase) resolveExisting(ctx context.Context, userID model.UserID) (*PrepareTwoFactorAuthOutput, error) {
	existing, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("2段階認証設定の取得に失敗: %w", err)
	}
	if existing == nil {
		return nil, nil
	}
	if existing.Enabled {
		return &PrepareTwoFactorAuthOutput{AlreadyEnabled: true}, nil
	}
	return &PrepareTwoFactorAuthOutput{Secret: existing.Secret}, nil
}
