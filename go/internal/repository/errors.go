package repository

import (
	"errors"

	sqlitedriver "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

// IsUniqueViolation reports whether err is the driver's error for a write that a
// UNIQUE constraint rejected.
//
// A caller uses it where losing such a race is a normal outcome rather than a
// fault: a value checked for availability and then written can be taken in
// between, and the constraint is what makes that visible. Recognizing the driver
// error is kept here so the layers above never import the driver to ask.
//
// Both result codes count as one answer: SQLite reports a rejected write against
// an INTEGER PRIMARY KEY (the rowid) as a primary-key violation and every other
// unique index as a unique violation, and the distinction says which index was
// hit, not what happened.
//
// [Ja] IsUniqueViolation は、err が UNIQUE 制約に拒否された書き込みに対するドライバの
// エラーかどうかを返します。
//
// 呼び出し側は、その競合に負けることが障害ではなく通常の結果である場面でこれを使います。
// 空きを確認してから書く値は、その間に取得されうるものであり、制約はそれを可視にする
// 仕組みです。ドライバのエラーを見分ける処理をここに置くのは、上位の層がこれを問うために
// ドライバを import せずに済むようにするためです。
//
// 2 つの結果コードを 1 つの答えとして扱います。SQLite は INTEGER PRIMARY KEY (rowid) に
// 対する拒否を主キー違反、それ以外の一意インデックスに対する拒否を一意制約違反として
// 報告しますが、この違いが示すのはどのインデックスに当たったかであって、何が起きたかでは
// ないためです。
func IsUniqueViolation(err error) bool {
	var sqliteErr *sqlitedriver.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	code := sqliteErr.Code()
	return code == sqlitelib.SQLITE_CONSTRAINT_UNIQUE || code == sqlitelib.SQLITE_CONSTRAINT_PRIMARYKEY
}
