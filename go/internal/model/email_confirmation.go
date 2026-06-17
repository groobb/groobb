package model

import "time"

// EmailConfirmationEvent names the flow an email confirmation belongs to, so one
// table can serve every flow that must verify an email address. It is a string
// (matching the event column) rather than an integer enum: Groobb stores such
// kinds as readable strings (the same choice as users.locale), so a row is
// self-describing in the database without a lookup.
//
// [Ja] EmailConfirmationEvent はメール確認がどのフローに属するかを表し、メール
// アドレスの検証が要る各フローを 1 つのテーブルで賄えるようにします。整数 enum では
// なく (event 列に合わせた) string とします。Groobb はこの種の区分を可読な文字列で
// 保持しており (users.locale と同じ選択)、DB 上で照合なしに行が自己記述的になります。
type EmailConfirmationEvent string

// EmailConfirmationEventSignUp is the confirmation issued during sign-up to
// verify a new account's email address. Other events (e.g. email change) are
// added when their flows are built.
//
// [Ja] EmailConfirmationEventSignUp はサインアップ時に新規アカウントの
// メールアドレスを検証するために発行される確認です。他のイベント (例: メール変更) は
// それぞれのフローを作る時点で追加します。
const EmailConfirmationEventSignUp EmailConfirmationEvent = "sign_up"

// EmailConfirmation is one verification code issued for an email address. Email
// is the address being verified and Event names the flow it was issued for;
// Code is the value the user must type back. The row is keyed by Email, not a
// UserID, because a confirmation is issued before the user exists (sign-up
// verifies the address first, then creates the user).
//
// StartedAt is when the code was issued and is the basis for its expiry window;
// for a freshly created row it equals CreatedAt. SucceededAt is nil until the
// code is accepted. FailedAttemptsCount counts wrong-code submissions and caps
// brute force: once it reaches the limit the confirmation stops being active.
//
// [Ja] EmailConfirmation はメールアドレスに対して発行された 1 つの確認コードです。
// Email は検証対象のアドレス、Event はそれが発行されたフローの名前で、Code は
// ユーザーが入力し返す値です。行は UserID ではなく Email をキーとします。確認は
// ユーザーが存在する前に発行される (サインアップはまずアドレスを検証し、その後
// ユーザーを作る) ためです。
//
// StartedAt はコードを発行した時刻で、有効期限ウィンドウの基準となります。新規作成
// された行では CreatedAt と一致します。SucceededAt はコードが受理されるまで nil です。
// FailedAttemptsCount は誤ったコードの送信回数で、総当たりを抑止します。上限に達すると
// 確認は active でなくなります。
type EmailConfirmation struct {
	ID                  EmailConfirmationID
	Email               string
	Event               EmailConfirmationEvent
	Code                string
	StartedAt           time.Time
	SucceededAt         *time.Time
	FailedAttemptsCount int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
