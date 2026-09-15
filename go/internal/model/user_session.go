package model

import "time"

// UserSessionはユーザーの1つのサインイン済み、CookieベースのDBセッション
// です。不透明なTokenがセッションCookieに保存される値で、リクエストをユーザーに
// 解決するにはTokenでセッションを引き、そのUserIDを読み出します。
//
// IPAddress / UserAgentはセッションを確立した場所を記録します。SignedInAtは
// サインインの時刻で、新規作成されたセッションではCreatedAtと一致します。
type UserSession struct {
	ID         UserSessionID
	UserID     UserID
	Token      string
	IPAddress  string
	UserAgent  string
	SignedInAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
