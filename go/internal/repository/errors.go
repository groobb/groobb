package repository

import (
	"errors"

	sqlitedriver "modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"
)

// IsUniqueViolationは、errがUNIQUE制約に拒否された書き込みに対するドライバの
// エラーかどうかを返します。
//
// 呼び出し側は、その競合に負けることが障害ではなく通常の結果である場面でこれを使います。
// 空きを確認してから書く値は、その間に取得されうるものであり、制約はそれを可視にする
// 仕組みです。ドライバのエラーを見分ける処理をここに置くのは、上位の層がこれを問うために
// ドライバをimportせずに済むようにするためです。
//
// 2つの結果コードを1つの答えとして扱います。SQLiteはINTEGER PRIMARY KEY (rowid) に
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
