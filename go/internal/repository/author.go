package repository

import "github.com/groobb/groobb/go/internal/model"

// The author of a thread or a post is nullable. A withdrawn account's row is
// eventually removed by the purge job, and the writing stays behind without it,
// so that the replies quoting it keep their context. Threads and posts convert
// that author the same way in both directions, and these two helpers are where
// the conversion lives instead of being repeated in each repository.
//
// [Ja] スレッドと投稿の作者は nullable である。退会したアカウントの行はいずれパージ
// ジョブが物理削除するが、書かれたものは作者を欠いたまま残る。それを引用した返信が
// 文脈を保てるようにするためである。スレッドと投稿はこの作者を双方向とも同じ形で変換
// するため、その変換を各リポジトリで繰り返さずにこの 2 つのヘルパーへ置く。

// rawAuthorID converts an author on its way into a query, returning nil for the
// writing whose author has withdrawn.
//
// [Ja] rawAuthorID は作者をクエリへ渡す方向で変換し、作者が退会した書き込みには nil を
// 返す。
func rawAuthorID(id *model.UserID) *int64 {
	if id == nil {
		return nil
	}
	raw := int64(*id)
	return &raw
}

// typedAuthorID converts an author on its way out of a query row, returning nil
// for the writing whose author has withdrawn.
//
// [Ja] typedAuthorID は作者をクエリの行から取り出す方向で変換し、作者が退会した
// 書き込みには nil を返す。
func typedAuthorID(raw *int64) *model.UserID {
	if raw == nil {
		return nil
	}
	id := model.UserID(*raw)
	return &id
}
