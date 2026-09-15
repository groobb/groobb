package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// moderationLogEntryは、モデレーションの操作1件が自身について記録することです。
// 操作者はそのフィールドに含まれません。誰が操作したかはrecordModerationLogに渡された
// Actorから読むため、呼び出し元が、別の誰かが行ったものとして操作を記録することは
// できません。
//
// 3つの対象のフィールドは、repository.CreateModerationLogInputが述べるとおり、Actionに
// 応じて埋めます。
type moderationLogEntry struct {
	Action       model.ModerationAction
	ThreadID     *model.ThreadID
	PostID       *model.PostID
	TargetUserID *model.UserID
	Reason       string
}

// recordModerationLogは、actorがいま行った操作について、履歴に1件を書き込みます。
//
// リポジトリは呼び出し元の書き込みトランザクションに参加していなければなりません
// (WithTx)。モデレーションのUseCaseはいずれもこの1つの関数を通して記録するため、操作が
// 起きたと述べる記録と、その操作が変えた行は、ともにコミットされるか、どちらもされないかの
// どちらかになります。
//
// groobbのサブコマンドを通して操作した運用者は、利用者idを持たない形で記録します。運用者を
// 記述する行がデータベースに無いためです。履歴はそのような記録を、コミュニティが名指せる
// 誰かを作者としない操作として読みます。これはパージされたアカウントが残す形でもあり、
// 履歴はどちらについても同じことを述べます。
func recordModerationLog(
	ctx context.Context,
	moderationLogRepo *repository.ModerationLogRepository,
	actor Actor,
	entry moderationLogEntry,
) error {
	var userID *model.UserID
	if !actor.isOperator {
		userID = &actor.userID
	}

	if _, err := moderationLogRepo.Create(ctx, repository.CreateModerationLogInput{
		UserID:       userID,
		Action:       entry.Action,
		ThreadID:     entry.ThreadID,
		PostID:       entry.PostID,
		TargetUserID: entry.TargetUserID,
		Reason:       entry.Reason,
	}); err != nil {
		return fmt.Errorf("操作履歴の記録に失敗: %w", err)
	}

	return nil
}
