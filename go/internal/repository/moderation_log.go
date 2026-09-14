package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// ModerationLogRepository reads and writes moderation logs through
// sqlc-generated queries.
//
// [Ja] ModerationLogRepositoryはsqlc生成のクエリ経由でmoderation_logsを読み書きします。
type ModerationLogRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewModerationLogRepository creates a ModerationLogRepository that reads
// through the database's read pool and writes through its write pool.
//
// [Ja] NewModerationLogRepositoryは、データベースの読み取り用プールで読み、書き込み用
// プールで書くModerationLogRepositoryを生成します。
func NewModerationLogRepository(db *database.DB) *ModerationLogRepository {
	return &ModerationLogRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new ModerationLogRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// Every moderation operation goes through this: the entry is written in the same
// transaction as the state it records, so a history that says an operation
// happened and a row that does not carry it cannot both be committed.
//
// [Ja] WithTxはqueriesをtx内で実行する新しいModerationLogRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
//
// モデレーションの操作はいずれもこれを通ります。記録は、それが記録する状態と同じ
// トランザクションで書かれるため、操作が起きたと述べる履歴と、それを持たない行が、
// ともにコミットされることはありません。
func (r *ModerationLogRepository) WithTx(tx *sql.Tx) *ModerationLogRepository {
	q := r.writer.WithTx(tx)
	return &ModerationLogRepository{reader: q, writer: q}
}

// CreateModerationLogInput holds the attributes of one recorded operation. id
// and the timestamps are assigned by the database, so the moment the history
// gives an operation comes from the same clock as the state the operation left
// behind.
//
// UserID is nil for an operator acting outside any account, and the three target
// fields are filled according to Action: the lock and the two unpublications name
// a thread (the post's unpublication naming its post as well), and a suspension
// names an account.
//
// [Ja] CreateModerationLogInputは記録する操作1件の属性を保持します。idとタイムスタンプは
// DB側で採番されるため、履歴が操作に与える時刻は、その操作が残した状態と同じ時計から
// 出ます。
//
// UserIDは、どのアカウントでもなく運用者として行った操作でnilになります。3つの対象の
// フィールドはActionに応じて埋めます。ロックと2つの非公開はスレッドを名指し (投稿の
// 非公開は併せてその投稿も名指します)、停止はアカウントを名指します。
type CreateModerationLogInput struct {
	UserID       *model.UserID
	Action       model.ModerationAction
	ThreadID     *model.ThreadID
	PostID       *model.PostID
	TargetUserID *model.UserID
	Reason       string
}

// Create records one operation and returns it with the database-assigned id and
// timestamps populated.
//
// The action values are not enumerated in a CHECK, because changing that
// constraint in SQLite would require rebuilding the table whenever an action is
// added. Instead, Create checks model.ModerationActions before inserting. A value
// outside that set indicates a caller naming an unsupported operation.
//
// [Ja] Createは操作1件を記録し、DBが採番したidとタイムスタンプを設定した状態で返します。
//
// actionの値域をCHECKで列挙すると、SQLiteでは操作を追加するたびに制約の変更のための
// テーブル再構築が必要になります。そのため、Createが挿入前にmodel.ModerationActionsで
// 検査します。値域外の値は、呼び出し元が未対応の操作を指定したことを意味します。
func (r *ModerationLogRepository) Create(ctx context.Context, input CreateModerationLogInput) (*model.ModerationLog, error) {
	if !input.Action.IsValid() {
		return nil, fmt.Errorf("操作履歴の操作の種類が不正: action=%q", input.Action)
	}

	row, err := r.writer.CreateModerationLog(ctx, query.CreateModerationLogParams{
		UserID:       rawLogUserID(input.UserID),
		Action:       string(input.Action),
		ThreadID:     rawLogThreadID(input.ThreadID),
		PostID:       rawLogPostID(input.PostID),
		TargetUserID: rawLogUserID(input.TargetUserID),
		Reason:       input.Reason,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// ListPage returns one page of the history, the most recent operation first.
//
// The order is the primary key descending rather than created_at descending.
// The two agree on when each row was written, since ids are issued in the order
// the rows are, and the primary key settles the operations that land in the same
// millisecond instead of leaving their order to the plan. It is also the index
// the table already has, so the page is read by walking it backwards rather than
// sorting the whole history to hand back fifty rows.
//
// [Ja] ListPageは履歴を1ページ分、新しい操作から順に返します。
//
// 並び順はcreated_atの降順ではなく主キーの降順です。idは行が書かれた順に採番されるため
// 2つはどちらが先に書かれたかについて一致し、そのうえで主キーは同じミリ秒に収まった操作の
// 順序を実行計画に委ねずに決めます。テーブルが既に持つ索引でもあるため、ページは履歴
// 全体を並べ替えて50行を返すのではなく、それを逆にたどって読まれます。
func (r *ModerationLogRepository) ListPage(ctx context.Context, limit, offset int) ([]*model.ModerationLog, error) {
	rows, err := r.reader.ListModerationLogsPage(ctx, query.ListModerationLogsPageParams{
		PageSize:   int64(limit),
		PageOffset: int64(offset),
	})
	if err != nil {
		return nil, err
	}

	logs := make([]*model.ModerationLog, len(rows))
	for i, row := range rows {
		logs[i] = r.toModel(row)
	}
	return logs, nil
}

// Count returns how many operations the history holds, so its pages can be
// numbered. Nothing is left out, because nothing is: an entry is never removed,
// and the one whose target has since been deleted still says that the operation
// happened.
//
// [Ja] Countは履歴が何件の操作を持つかを返し、ページに番号を振れるようにします。何も
// 除きません。除くものが無いためです。記録は削除されることが無く、対象がその後に削除された
// 記録も、その操作が起きたことを述べ続けます。
func (r *ModerationLogRepository) Count(ctx context.Context) (int, error) {
	count, err := r.reader.CountModerationLogs(ctx)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// toModel converts a query.ModerationLog row into a model.ModerationLog, casting
// the raw ids into their typed forms and the stored timestamps back into
// time.Time at the repository boundary.
//
// [Ja] toModelはquery.ModerationLogをmodel.ModerationLogに変換し、リポジトリの境界で
// 生のidを型付きの形に、保存書式の時刻をtime.Timeにキャストします。
func (r *ModerationLogRepository) toModel(row query.ModerationLog) *model.ModerationLog {
	return &model.ModerationLog{
		ID:           model.ModerationLogID(row.ID),
		UserID:       typedLogUserID(row.UserID),
		Action:       model.ModerationAction(row.Action),
		ThreadID:     typedLogThreadID(row.ThreadID),
		PostID:       typedLogPostID(row.PostID),
		TargetUserID: typedLogUserID(row.TargetUserID),
		Reason:       row.Reason,

		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}

// The operator and the three targets of a moderation log are all nullable: the
// operator has no row when the operation was performed outside any account, and
// a target turns NULL once the row it points at is physically deleted. The three
// pairs below are where those conversions live, so Create and toModel read as
// the assignments they are rather than as a run of nil checks.
//
// [Ja] 操作履歴の操作者と3つの対象はいずれもnullableである。どのアカウントでもなく
// 行われた操作の操作者は指す行を持たず、対象は指している行が物理削除された時点でNULLに
// なる。以下の3つの対は、それらの変換を置く場所であり、CreateとtoModelがnil検査の
// 連なりではなく、そのままの代入として読めるようにする。

// rawLogUserID converts an account on its way into a query, returning nil for an
// operation no account is behind and for a target no longer there.
//
// [Ja] rawLogUserIDはアカウントをクエリへ渡す方向で変換し、どのアカウントも背後に
// 持たない操作と、もう存在しない対象にはnilを返す。
func rawLogUserID(id *model.UserID) *int64 {
	if id == nil {
		return nil
	}
	raw := int64(*id)
	return &raw
}

// typedLogUserID converts an account on its way out of a query row.
//
// [Ja] typedLogUserIDはアカウントをクエリの行から取り出す方向で変換する。
func typedLogUserID(raw *int64) *model.UserID {
	if raw == nil {
		return nil
	}
	id := model.UserID(*raw)
	return &id
}

// rawLogThreadID converts a thread on its way into a query, returning nil for an
// operation aimed at no thread.
//
// [Ja] rawLogThreadIDはスレッドをクエリへ渡す方向で変換し、スレッドを対象としない操作
// にはnilを返す。
func rawLogThreadID(id *model.ThreadID) *int64 {
	if id == nil {
		return nil
	}
	raw := int64(*id)
	return &raw
}

// typedLogThreadID converts a thread on its way out of a query row.
//
// [Ja] typedLogThreadIDはスレッドをクエリの行から取り出す方向で変換する。
func typedLogThreadID(raw *int64) *model.ThreadID {
	if raw == nil {
		return nil
	}
	id := model.ThreadID(*raw)
	return &id
}

// rawLogPostID converts a post on its way into a query, returning nil for an
// operation aimed at no post.
//
// [Ja] rawLogPostIDは投稿をクエリへ渡す方向で変換し、投稿を対象としない操作にはnilを
// 返す。
func rawLogPostID(id *model.PostID) *int64 {
	if id == nil {
		return nil
	}
	raw := int64(*id)
	return &raw
}

// typedLogPostID converts a post on its way out of a query row.
//
// [Ja] typedLogPostIDは投稿をクエリの行から取り出す方向で変換する。
func typedLogPostID(raw *int64) *model.PostID {
	if raw == nil {
		return nil
	}
	id := model.PostID(*raw)
	return &id
}
