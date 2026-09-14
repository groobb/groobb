package model

import (
	"slices"
	"time"
)

// ModerationAction names one operation an administrator performed. It is the
// value stored in moderation_logs.action, so what the history holds reads back
// as the same words this file writes down.
//
// [Ja] ModerationActionは管理者が行った操作を1つ名指します。moderation_logs.actionに
// 保存される値そのものであるため、履歴が保持するものは、このファイルが書き下すのと
// 同じ言葉として読み戻されます。
type ModerationAction string

const (
	// ModerationActionThreadLock and ModerationActionThreadUnlock are the two
	// halves of the moderator's lock. They are separate values rather than one
	// value with a direction, because the history is read as a list of what
	// happened and a reader should not have to hold the previous row in mind to
	// know which way this one went.
	//
	// [Ja] ModerationActionThreadLockとModerationActionThreadUnlockは、管理者による
	// ロックの2つの向きです。向きを添えた1つの値ではなく別々の値にしているのは、履歴が
	// 起きたことの一覧として読まれるものであり、ある行がどちらの向きだったかを知るために
	// 直前の行を覚えておく必要が生じないようにするためです。
	ModerationActionThreadLock   ModerationAction = "thread_lock"
	ModerationActionThreadUnlock ModerationAction = "thread_unlock"

	// ModerationActionThreadUnpublish and ModerationActionPostUnpublish are the
	// two unpublications. They are told apart by the action rather than by which
	// of the target columns is filled, so a reader of the history names the
	// operation without inspecting the row's shape.
	//
	// [Ja] ModerationActionThreadUnpublishとModerationActionPostUnpublishは2つの
	// 非公開です。どの対象の列が埋まっているかではなく操作の種類で区別するため、履歴を
	// 読む側は行の形を調べずに操作を名指せます。
	ModerationActionThreadUnpublish ModerationAction = "thread_unpublish"
	ModerationActionPostUnpublish   ModerationAction = "post_unpublish"

	// ModerationActionUserSuspend and ModerationActionUserUnsuspend are the two
	// halves of a suspension, told apart for the reason the lock's two halves
	// are.
	//
	// [Ja] ModerationActionUserSuspendとModerationActionUserUnsuspendは停止の2つの
	// 向きで、ロックの2つの向きと同じ理由で区別します。
	ModerationActionUserSuspend   ModerationAction = "user_suspend"
	ModerationActionUserUnsuspend ModerationAction = "user_unsuspend"
)

// ModerationActions returns every operation the history can record. It is the
// one place the set is written out, and the repository applies it before an
// insert.
//
// A fresh slice is returned per call so a caller cannot edit the set out from
// under the others.
//
// [Ja] ModerationActionsは、履歴が記録しうる操作をすべて返します。値域を書き下す唯一の
// 場所であり、リポジトリが挿入の前にこれを適用します。
//
// 呼び出しごとに新しいスライスを返すため、ある呼び出し側が他から見える集合を書き換えて
// しまうことはありません。
func ModerationActions() []ModerationAction {
	return []ModerationAction{
		ModerationActionThreadLock,
		ModerationActionThreadUnlock,
		ModerationActionThreadUnpublish,
		ModerationActionPostUnpublish,
		ModerationActionUserSuspend,
		ModerationActionUserUnsuspend,
	}
}

// IsValid reports whether a is one of the operations the history can record.
//
// The moderation_logs.action column lists no values in a CHECK, for the reason
// threads.language lists none: SQLite cannot alter one, so every operation added
// would take a migration that rebuilds the table and copies its rows. The set is
// held here instead, and the repository applies this before the insert, so a
// value outside it is refused at the write a CHECK would have refused it at.
//
// [Ja] IsValidは、aが履歴の記録しうる操作のいずれかであるかを返します。
//
// moderation_logs.action列が値をCHECKで列挙しないのはthreads.languageと同じ理由に
// よります。SQLiteはCHECKを変更できず、操作を1つ足すたびにテーブルを作り直して行を移す
// マイグレーションが要るためです。値域はここに持ち、リポジトリが挿入の前にこれを
// 適用します。集合の外の値は、CHECKがあれば拒否されていたのと同じ書き込みで拒否されます。
func (a ModerationAction) IsValid() bool {
	return slices.Contains(ModerationActions(), a)
}

// ModerationLog is one operation an administrator performed, kept so that what
// was done to a thread, a post or an account is answerable from something other
// than the state the operation left behind.
//
// [Ja] ModerationLogは管理者が行った操作1件です。スレッド・投稿・アカウントに何が
// 行われたかを、その操作が残した状態以外からも答えられるように保持します。
type ModerationLog struct {
	ID ModerationLogID

	// UserID is the administrator who acted, and is nil for two situations that
	// are deliberately not told apart: an operator working outside any account
	// has no row to point at, and a withdrawn account's row is eventually removed
	// by the purge job. An account is anonymized at the moment it withdraws, so
	// nothing readable is lost when the reference goes.
	//
	// [Ja] UserIDは操作した管理者で、意図的に区別しない2つの状況でnilになります。どの
	// アカウントでもなく運用者として行った操作は指す行を持たず、退会したアカウントの行は
	// いずれパージジョブが物理削除します。アカウントは退会した時点で匿名化されるため、
	// 参照が外れても読み取れるものは失われません。
	UserID *UserID

	Action ModerationAction

	// ThreadID, PostID and TargetUserID name what the operation was aimed at, one
	// field per kind of target rather than a type and an id read together. Which
	// of them an operation fills follows from its Action: the lock and the two
	// unpublications name a thread (the post's unpublication naming its post as
	// well), and a suspension names an account.
	//
	// Each may also turn nil after the row it points at is physically deleted,
	// which is what happens to a thread and its posts when their board is
	// deleted.
	//
	// [Ja] ThreadID・PostID・TargetUserIDは操作が何に向けられたかを名指すもので、種類と
	// idを組で読むのではなく、対象の種類ごとに1つのフィールドを持ちます。ある操作がどれを
	// 埋めるかはActionから決まります。ロックと2つの非公開はスレッドを名指し (投稿の非公開は
	// 併せてその投稿も名指します)、停止はアカウントを名指します。
	//
	// いずれも、指している行が物理削除された後にnilになることがあります。掲示板が削除された
	// ときのスレッドとその投稿がこれに当たります。
	ThreadID     *ThreadID
	PostID       *PostID
	TargetUserID *UserID

	// Reason is what the administrator wrote about the operation, and is the
	// empty string when they wrote nothing. It is shown in the admin screens
	// alone: it explains the decision to whoever reviews the history, not to the
	// community.
	//
	// [Ja] Reasonは管理者がその操作について書いたもので、何も書かなかったときは空文字列
	// です。表示するのは管理画面だけです。判断を説明する相手は履歴を確かめる側であって、
	// コミュニティではありません。
	Reason string

	CreatedAt time.Time
	UpdatedAt time.Time
}
