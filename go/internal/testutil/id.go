package testutil

// UnusedID is a positive database id high enough that no test row is ever
// assigned it. Tests sign it into a continuation token when they need a
// well-formed handoff that resolves to no row.
//
// [Ja] UnusedID はテストの行に割り当てられない十分大きな正のデータベース id です。
// テストは、整形式だがどの行にも解決しない受け渡しが必要なとき、これを continuation
// token へ署名して使います。
const UnusedID int64 = 999999
