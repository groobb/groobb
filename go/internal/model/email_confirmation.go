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
// verify a new account's email address. Other events are added when their flows
// are built.
//
// [Ja] EmailConfirmationEventSignUp はサインアップ時に新規アカウントの
// メールアドレスを検証するために発行される確認です。他のイベントはそれぞれの
// フローを作る時点で追加します。
const EmailConfirmationEventSignUp EmailConfirmationEvent = "sign_up"

// EmailConfirmationEventEmailChange is the confirmation issued when a logged-in
// user changes their email address, proving control of the new address before it
// replaces the current one.
//
// [Ja] EmailConfirmationEventEmailChange はログイン済みユーザーがメールアドレスを
// 変更するときに発行される確認です。新しいアドレスが現在のアドレスを置き換える前に、
// その管理権を証明します。
const EmailConfirmationEventEmailChange EmailConfirmationEvent = "email_change"

// EmailConfirmation is one verification code issued for an email address. Email
// is the address being verified and Event names the flow it was issued for;
// Code is the value the user must type back. UserID ties the confirmation to a
// user when one already exists: it is nil for sign-up (the address is verified
// before the user is created) and set for email change (a logged-in user
// requests it), which lets the email-change flow look up a user's pending
// confirmation directly.
//
// StartedAt is when the code was issued and is the basis for its expiry window;
// for a freshly created row it equals CreatedAt. SucceededAt is nil until the
// code is accepted. FailedAttemptsCount counts wrong-code submissions and caps
// brute force: once it reaches the limit the confirmation stops being active.
//
// [Ja] EmailConfirmation はメールアドレスに対して発行された 1 つの確認コードです。
// Email は検証対象のアドレス、Event はそれが発行されたフローの名前で、Code は
// ユーザーが入力し返す値です。UserID は、ユーザーが既に存在する場合に確認をその
// ユーザーへ紐付けます。サインアップでは nil で (アドレスはユーザー作成前に検証される)、
// メール変更では設定されます (ログイン済みユーザーが申請する)。これによりメール変更
// フローはユーザーの保留中の確認を直接引けます。
//
// StartedAt はコードを発行した時刻で、有効期限ウィンドウの基準となります。新規作成
// された行では CreatedAt と一致します。SucceededAt はコードが受理されるまで nil です。
// FailedAttemptsCount は誤ったコードの送信回数で、総当たりを抑止します。上限に達すると
// 確認は active でなくなります。
type EmailConfirmation struct {
	ID                  EmailConfirmationID
	UserID              *UserID
	Email               string
	Event               EmailConfirmationEvent
	Code                string
	StartedAt           time.Time
	SucceededAt         *time.Time
	FailedAttemptsCount int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
