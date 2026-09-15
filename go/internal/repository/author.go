package repository

import "github.com/groobb/groobb/go/internal/model"

// スレッドと投稿の作者はnullableである。退会したアカウントの行はいずれパージ
// ジョブが物理削除するが、書かれたものは作者を欠いたまま残る。それを引用した返信が
// 文脈を保てるようにするためである。スレッドと投稿はこの作者を双方向とも同じ形で変換
// するため、その変換を各リポジトリで繰り返さずにこの2つのヘルパーへ置く。

// rawAuthorIDは作者をクエリへ渡す方向で変換し、作者が退会した書き込みにはnilを
// 返す。
func rawAuthorID(id *model.UserID) *int64 {
	if id == nil {
		return nil
	}
	raw := int64(*id)
	return &raw
}

// typedAuthorIDは作者をクエリの行から取り出す方向で変換し、作者が退会した
// 書き込みにはnilを返す。
func typedAuthorID(raw *int64) *model.UserID {
	if raw == nil {
		return nil
	}
	id := model.UserID(*raw)
	return &id
}
