package testutil

// UnusedIDはテストの行に割り当てられない十分大きな正のデータベースidです。
// テストは、整形式だがどの行にも解決しない受け渡しが必要なとき、これをcontinuation
// tokenへ署名して使います。
const UnusedID int64 = 999999
