package model

import "time"

// User is the global, canonical identity that authentication hangs off of. It
// holds identity-level attributes: email (also the contact address), a globally
// unique atname (the @handle identifying who a user is), and the account-level
// locale and time zone used when rendering messages outside of any space context
// (e.g. password-reset emails rendered asynchronously).
//
// atname lives here on the global user, not per space: it is a stable, globally
// unique handle so a person is the same identity in every space (ADR 0003). A
// display name and role are deliberately not modeled yet; until the need arises
// the atname doubles as the name shown for a user. The password digest is kept
// in a separate credentials table, so it is absent here.
//
// [Ja] User は認証がぶら下がるグローバルで正準な身元。身元レベルの属性を持つ。すなわち
// email (連絡先も兼ねる)、グローバルに一意な atname (ユーザーが何者かを示す @ハンドル)、
// そしてスペース文脈の外でメッセージを描画するときに使うアカウントレベルの
// locale / time zone (例: 非同期に描画するパスワードリセットメール) である。
//
// atname はスペース単位ではなくこのグローバルな user が持つ。安定したグローバルに一意な
// ハンドルとし、どのスペースでも同一人物が同一の身元になるようにするためである
// (ADR 0003)。表示名やロールは意図的にまだモデル化しておらず、必要が生じるまでは atname を
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
