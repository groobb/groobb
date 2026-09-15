package model

import "time"

// EmailConfirmationEventはメール確認がどのフローに属するかを表し、メール
// アドレスの検証が要る各フローを1つのテーブルで賄えるようにします。整数enumでは
// なく (event列に合わせた) stringとします。Groobbはこの種の区分を可読な文字列で
// 保持しており (users.localeと同じ選択)、DB上で照合なしに行が自己記述的になります。
type EmailConfirmationEvent string

// EmailConfirmationEventSignUpはサインアップ時に新規アカウントの
// メールアドレスを検証するために発行される確認です。他のイベントはそれぞれの
// フローを作る時点で追加します。
const EmailConfirmationEventSignUp EmailConfirmationEvent = "sign_up"

// EmailConfirmationEventEmailChangeはログイン済みユーザーがメールアドレスを
// 変更するときに発行される確認です。新しいアドレスが現在のアドレスを置き換える前に、
// その管理権を証明します。
const EmailConfirmationEventEmailChange EmailConfirmationEvent = "email_change"

// EmailConfirmationはメールアドレスに対して発行された1つの確認コードです。
// Emailは検証対象のアドレス、Eventはそれが発行されたフローの名前で、Codeは
// ユーザーが入力し返す値です。UserIDは、ユーザーが既に存在する場合に確認をその
// ユーザーへ紐付けます。サインアップではnilで (アドレスはユーザー作成前に検証される)、
// メール変更では設定されます (ログイン済みユーザーが申請する)。これによりメール変更
// フローはユーザーの保留中の確認を直接引けます。
//
// StartedAtはコードを発行した時刻で、有効期限ウィンドウの基準となります。新規作成
// された行ではCreatedAtと一致します。SucceededAtはコードが受理されるまでnilです。
// FailedAttemptsCountは誤ったコードの送信回数で、総当たりを抑止します。上限に達すると
// 確認はactiveでなくなります。
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
