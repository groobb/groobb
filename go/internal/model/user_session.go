package model

import "time"

// UserSession is one signed-in, Cookie-backed database session for a user. The
// opaque Token is what the session cookie stores; resolving a request to a user
// means looking the session up by Token and then loading its UserID.
//
// IPAddress and UserAgent record where the session was established. SignedInAt
// is the sign-in moment; for a freshly created session it equals CreatedAt.
//
// [Ja] UserSession はユーザーの 1 つのサインイン済み、Cookie ベースの DB セッション
// です。不透明な Token がセッション Cookie に保存される値で、リクエストをユーザーに
// 解決するには Token でセッションを引き、その UserID を読み出します。
//
// IPAddress / UserAgent はセッションを確立した場所を記録します。SignedInAt は
// サインインの時刻で、新規作成されたセッションでは CreatedAt と一致します。
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
