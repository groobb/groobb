package model

import (
	"errors"
	"fmt"
	"time"
)

// ValidationErrorは入力バリデーションの失敗の集合を表します。これを受け取った
// ハンドラーは、送信されたフォームを該当フィールドのメッセージ付きで再描画します
// (HTTP 422)。
type ValidationError struct {
	// Globalは特定のフィールドではなくフォーム全体に関わるメッセージを集めます。
	Global []string
	// Fieldsはフィールドごとに紐づくメッセージを保持します。
	Fields map[string][]string
}

// ErrorはValidationErrorをerrorインターフェースに適合させます。ユーザー
// 向けメッセージではなく固定の内部文字列を返します。ハンドラーが描画するのは
// Fields / Globalのフィールド別メッセージです。
func (e *ValidationError) Error() string { return "validation failed" }

// AddGlobalはフォーム全体のエラーメッセージを追加します。
func (e *ValidationError) AddGlobal(message string) {
	e.Global = append(e.Global, message)
}

// AddFieldは指定したフィールドのエラーメッセージを追加します。
func (e *ValidationError) AddField(field, message string) {
	if e.Fields == nil {
		e.Fields = make(map[string][]string)
	}
	e.Fields[field] = append(e.Fields[field], message)
}

// HasErrorsはグローバルまたはフィールドのエラーが追加されているかを返します。
func (e *ValidationError) HasErrors() bool {
	if e == nil {
		return false
	}
	return len(e.Global) > 0 || len(e.Fields) > 0
}

// HasGlobalErrorはフォーム全体のエラーが追加されているかを返します。
func (e *ValidationError) HasGlobalError() bool {
	if e == nil {
		return false
	}
	return len(e.Global) > 0
}

// GetGlobalErrorsはフォーム全体のエラーメッセージを返します。
func (e *ValidationError) GetGlobalErrors() []string {
	if e == nil {
		return nil
	}
	return e.Global
}

// HasFieldErrorsは、どのフィールドであれエラーを持つフィールドがあるかを返します。
// フォームが直すべきものを並べる要約を描くのは、フィールドの中に直すものがあるときだけ
// であり、それは送信が全体として拒否されたかどうかとは別の問いです。
func (e *ValidationError) HasFieldErrors() bool {
	if e == nil {
		return false
	}
	return len(e.Fields) > 0
}

// HasFieldErrorは指定したフィールドにエラーがあるかを返します。
func (e *ValidationError) HasFieldError(field string) bool {
	if e == nil || e.Fields == nil {
		return false
	}
	return len(e.Fields[field]) > 0
}

// GetFieldErrorsは指定したフィールドのエラーメッセージを返します。
func (e *ValidationError) GetFieldErrors(field string) []string {
	if e == nil || e.Fields == nil {
		return nil
	}
	return e.Fields[field]
}

// FieldErrorはテンプレートで反復するために平坦化した単一のフィールドエラー
// です。
type FieldError struct {
	Field   string
	Message string
}

// FieldErrorsはすべてのフィールドエラーを反復可能なスライスに平坦化して返し
// ます。
func (e *ValidationError) FieldErrors() []FieldError {
	if e == nil || e.Fields == nil {
		return nil
	}
	var errs []FieldError
	for field, messages := range e.Fields {
		for _, message := range messages {
			errs = append(errs, FieldError{
				Field:   field,
				Message: message,
			})
		}
	}
	return errs
}

// NewValidationErrorはメッセージを集める準備が整った空のValidationErrorを
// 生成します。
func NewValidationError() *ValidationError {
	return &ValidationError{
		Global: []string{},
		Fields: make(map[string][]string),
	}
}

// AppErrorCodeはアプリケーションエラーを分類し、ハンドラーがHTTPステータス
// コードに対応づけられるようにします。
type AppErrorCode int

const (
	// AppErrCodeResourceNotFoundはリソース未存在 (404相当) です。
	AppErrCodeResourceNotFound AppErrorCode = iota + 1
	// AppErrCodeResourceUnpublishedは、管理者が見えない場所へ移したリソース
	// (404相当) です。同じステータスで応答するAppErrCodeResourceNotFoundと分けているのは、
	// ここではハンドラーに述べることがもう1つあるためです。そのアドレスは、コミュニティが
	// 一度も持たなかったものではなく、持っていて今は示さないものであるため、訪問者には
	// スレッドが取り下げられたことを伝えます。辿ったリンクが打ち間違いだったのかどうかを
	// 考えさせずに済みます。
	AppErrCodeResourceUnpublished
	// AppErrCodeForbiddenは権限不足 (403相当) です。
	AppErrCodeForbidden
	// AppErrCodeConflictは状態の競合 (409相当) です。
	AppErrCodeConflict
	// AppErrCodeThreadLockedは、書き込み先のスレッドがそれ以上の投稿を受け付けない
	// ために拒否した投稿 (409相当) です。同じステータスで応答するAppErrCodeConflictと
	// 分けているのは、ここではハンドラーに述べることがもう1つあるためです。スレッドが
	// ロックされている理由がエラーとともに運ばれ、フォームの代わりに示す案内になります。
	AppErrCodeThreadLocked
	// AppErrCodeRateLimitedは、前の要求から間を置かずに届いたために拒否した
	// 要求 (429相当) です。
	AppErrCodeRateLimited
	// AppErrCodeInternalは想定済みの内部エラー (500相当) です。
	AppErrCodeInternal
)

// AppErrorは業務レベルの既知の失敗を表します (SafeErrorパターン)。Error() は
// ユーザー安全なメッセージのみを返すため、内部原因がerrorインターフェース経由で
// ユーザーに漏れることはありません。
type AppError struct {
	// Codeはハンドラーがステータスコードを決めるために使うエラー種別です。
	Code AppErrorCode
	// UserMsgはユーザー安全なメッセージです。内部情報を含めてはなりません。
	UserMsg string
	// Internalはログ出力用の内部エラーです。ユーザーには公開しません。
	Internal error
	// Metadataはuser_idやresource_idなどの構造化ログ用コンテキストです。
	Metadata map[string]string
	// RetryAfterは、拒否された要求をもう一度試す価値が出るまでに呼び出し元が待つ
	// 時間です。AppErrCodeRateLimitedに伴うもので、それ以外では0です。
	//
	// Metadataの項目ではなくフィールドなのは、ハンドラーがこの値に基づいて動くため
	// です。待ち時間はRetry-Afterヘッダーになり、利用者に伝える秒数になります。
	// Metadataはログのために書くものであり、そこから取り戻した時間は、使う前に文字列
	// から解析し直すことになります。
	RetryAfter time.Duration
	// LockReasonsは、スレッドが投稿を拒否した理由です。AppErrCodeThreadLockedに
	// 伴うもので、それ以外では空です。
	//
	// 理由はUserMsgやMetadataから読み取り直すのではなく、Thread.LockReasonsが生んだ
	// 型付きの値のまま運びます。ハンドラーは示す案内をこれらから選び、そのうちの1つ
	// (投稿数の上限) は、その案内が次のスレッドを差し出すかどうかも決めます。人が読むために
	// 書かれた文章も、ログのために書かれたメタデータも、どちらの問いに答えるにも解析を
	// 挟むことになります。
	LockReasons []ThreadLockReason
}

// Errorはユーザー安全なメッセージのみを返します。
func (e *AppError) Error() string { return e.UserMsg }

// Unwrapは内部エラーをerrors.Is / errors.Asのチェーンに公開します。
func (e *AppError) Unwrap() error { return e.Internal }

// LogStringは内部原因とメタデータを含む、ログ専用の詳細表現を返します。
func (e *AppError) LogString() string {
	return fmt.Sprintf("Code: %d | Msg: %s | Cause: %v | Meta: %v",
		e.Code, e.UserMsg, e.Internal, e.Metadata)
}

// AsValidationErrorはerrから *ValidationErrorを取り出します。errがそれで
// ない (ラップもしていない) 場合はnilを返します。
func AsValidationError(err error) *ValidationError {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve
	}
	return nil
}

// AsAppErrorはerrから *AppErrorを取り出します。errがそれでない (ラップも
// していない) 場合はnilを返します。
func AsAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}
