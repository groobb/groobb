package model

import (
	"fmt"
	"time"
)

// User is the canonical identity that authentication hangs off of: one per
// account on this instance. It holds identity-level attributes: email (also
// the contact address), an atname unique within the instance (the @handle
// identifying who a user is), and the account-level locale and time zone used
// when rendering messages outside of a request (e.g. password-reset emails
// rendered asynchronously by a job).
//
// An instance hosts exactly one community (ADR 0006), so there is no unit below
// it to scope an identity to, and the uniqueness of an atname closes at the
// instance boundary (ADR 0007). A display name is deliberately not modeled yet;
// until the need arises the atname doubles as the name shown for a user. The
// password digest is kept in a separate credentials table, so it is absent
// here.
//
// [Ja] User は認証がぶら下がる正準な身元で、このインスタンス上のアカウント 1 つに
// つき 1 つ存在する。身元レベルの属性を持つ。すなわち email (連絡先も兼ねる)、
// インスタンス内で一意な atname (ユーザーが何者かを示す @ハンドル)、そしてリクエストの
// 外でメッセージを描画するときに使うアカウントレベルの locale / time zone
// (例: ジョブが非同期に描画するパスワードリセットメール) である。
//
// 1 インスタンスはちょうど 1 つのコミュニティを運営する (ADR 0006) ため、身元を
// インスタンスより下の単位に閉じ込める器が無く、atname の一意性はインスタンスの境界で
// 閉じる (ADR 0007)。表示名は意図的にまだモデル化しておらず、必要が生じるまでは atname を
// ユーザーの表示名としても用いる。パスワードダイジェストは別の資格情報テーブルに保持する
// ため、ここには無い。
type User struct {
	ID       UserID
	Email    string
	Atname   string
	Locale   string
	TimeZone string

	// DeletedAt marks a withdrawn account: nil means the account is active, and a
	// non-nil time is the moment the user withdrew. Withdrawal soft-deletes the row
	// (setting this alongside anonymizing email/atname) so the account is inert
	// right away, while the physical delete is left to a later purge job.
	// Authentication lookups exclude rows where this is non-nil, so a withdrawn
	// user never resolves back into a session.
	//
	// [Ja] DeletedAt は退会済みアカウントを表す。nil はアカウントがアクティブであること、
	// 非 nil はユーザーが退会した時刻を意味する。退会はこの値をセットして (email / atname の
	// 匿名化と同時に) 行を論理削除し、アカウントを即座に無効化する。物理削除は後続のパージ
	// ジョブに委ねる。認証系のルックアップはこの値が非 nil の行を除外するため、退会済み
	// ユーザーがセッションとして再び解決されることはない。
	DeletedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// AnonymizedEmail derives the placeholder email a withdrawn account's email is
// overwritten with. It embeds the user id so the value is distinct per account,
// freeing the original address for re-registration.
//
// The reserved .invalid TLD (RFC 2606) is what makes the value unreachable: both
// sign-up and an email change require a confirmation code delivered to the
// address, and .invalid can never receive one, so no account can hold this value
// and the overwrite cannot lose the users.email UNIQUE constraint to a live row.
//
// [Ja] AnonymizedEmail は退会済みアカウントの email を上書きする代替 email を導出します。
// ユーザー id を埋め込むことで値をアカウントごとに別のものにし、元のアドレスを再登録用に
// 解放します。
//
// 値を到達不能にしているのは予約 TLD の .invalid (RFC 2606) です。サインアップもメール
// アドレス変更も、アドレスへ配送された確認コードを要求しますが、.invalid はそれを受け取れ
// ません。そのためこの値を保持できるアカウントは存在せず、上書きが実在の行との間で
// users.email の UNIQUE 制約に負けることがありません。
func AnonymizedEmail(userID UserID) string {
	return fmt.Sprintf("deleted-%s@deleted.invalid", userID.String())
}

// AnonymizedAtname derives the placeholder atname a withdrawn account's atname is
// overwritten with. It embeds the user id so the value is distinct per account,
// freeing the original atname for reuse.
//
// The hyphen is what makes the value unreachable: it is outside the atname
// character set of ASCII letters, digits, and underscore, so no account can ever
// hold this value and the overwrite cannot lose the users.atname UNIQUE
// constraint to a live row. A separator inside that character set would not be
// enough, because an id in decimal is short enough to spell a tombstone that
// passes the account form and squats the value the owner's withdrawal needs.
// users.atname is NOCASE-collated TEXT with no length bound or format check, so
// the column accepts the hyphen; this tombstone value is never re-validated or
// shown as a handle.
//
// [Ja] AnonymizedAtname は退会済みアカウントの atname を上書きする代替 atname を導出
// します。ユーザー id を埋め込むことで値をアカウントごとに別のものにし、元の atname を
// 再利用向けに解放します。
//
// 値を到達不能にしているのはハイフンです。ハイフンは atname の文字集合である ASCII
// 英数字 / アンダースコアの外にあるため、この値を保持できるアカウントは存在せず、上書きが
// 実在の行との間で users.atname の UNIQUE 制約に負けることがありません。区切りを文字集合
// 内の文字にするとこれは成り立ちません。10 進表記の id は短く、アカウントフォームを通る
// 墓標値を綴れてしまうため、退会に必要な値を先取りされうるからです。users.atname は長さ
// 上限も形式チェックも無い NOCASE 照合の TEXT のため、カラム自体はハイフンを受け付けます。
// この墓標値は再検証もハンドルとしての表示もされません。
func AnonymizedAtname(userID UserID) string {
	return "deleted-" + userID.String()
}
