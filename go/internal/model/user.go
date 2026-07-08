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
	ID        UserID
	Email     string
	Atname    string
	Locale    string
	TimeZone  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
