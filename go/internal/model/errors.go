package model

import (
	"errors"
	"fmt"
)

// ValidationError represents a set of input validation failures. A handler that
// receives it re-renders the submitted form (HTTP 422) with the messages shown
// against the relevant fields.
//
// [Ja] ValidationError は入力バリデーションの失敗の集合を表します。これを受け取った
// ハンドラーは、送信されたフォームを該当フィールドのメッセージ付きで再描画します
// (HTTP 422)。
type ValidationError struct {
	// Global collects messages that apply to the form as a whole rather than to
	// a single field.
	//
	// [Ja] Global は特定のフィールドではなくフォーム全体に関わるメッセージを集めます。
	Global []string
	// Fields collects messages keyed by the field they belong to.
	//
	// [Ja] Fields はフィールドごとに紐づくメッセージを保持します。
	Fields map[string][]string
}

// Error makes ValidationError satisfy the error interface. It returns a fixed
// internal string, never a user-facing message; the per-field messages in
// Fields / Global are what handlers render.
//
// [Ja] Error は ValidationError を error インターフェースに適合させます。ユーザー
// 向けメッセージではなく固定の内部文字列を返します。ハンドラーが描画するのは
// Fields / Global のフィールド別メッセージです。
func (e *ValidationError) Error() string { return "validation failed" }

// AddGlobal appends a form-wide error message.
//
// [Ja] AddGlobal はフォーム全体のエラーメッセージを追加します。
func (e *ValidationError) AddGlobal(message string) {
	e.Global = append(e.Global, message)
}

// AddField appends an error message for the given field.
//
// [Ja] AddField は指定したフィールドのエラーメッセージを追加します。
func (e *ValidationError) AddField(field, message string) {
	if e.Fields == nil {
		e.Fields = make(map[string][]string)
	}
	e.Fields[field] = append(e.Fields[field], message)
}

// HasErrors reports whether any global or field error has been added.
//
// [Ja] HasErrors はグローバルまたはフィールドのエラーが追加されているかを返します。
func (e *ValidationError) HasErrors() bool {
	if e == nil {
		return false
	}
	return len(e.Global) > 0 || len(e.Fields) > 0
}

// HasFieldError reports whether the given field has any error.
//
// [Ja] HasFieldError は指定したフィールドにエラーがあるかを返します。
func (e *ValidationError) HasFieldError(field string) bool {
	if e == nil || e.Fields == nil {
		return false
	}
	return len(e.Fields[field]) > 0
}

// GetFieldErrors returns the error messages for the given field.
//
// [Ja] GetFieldErrors は指定したフィールドのエラーメッセージを返します。
func (e *ValidationError) GetFieldErrors(field string) []string {
	if e == nil || e.Fields == nil {
		return nil
	}
	return e.Fields[field]
}

// FieldError is a single field error flattened for template iteration.
//
// [Ja] FieldError はテンプレートで反復するために平坦化した単一のフィールドエラー
// です。
type FieldError struct {
	Field   string
	Message string
}

// FieldErrors returns all field errors flattened into an iterable slice.
//
// [Ja] FieldErrors はすべてのフィールドエラーを反復可能なスライスに平坦化して返し
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

// NewValidationError creates an empty ValidationError ready to collect messages.
//
// [Ja] NewValidationError はメッセージを集める準備が整った空の ValidationError を
// 生成します。
func NewValidationError() *ValidationError {
	return &ValidationError{
		Global: []string{},
		Fields: make(map[string][]string),
	}
}

// AppErrorCode classifies an application error so that a handler can map it to
// an HTTP status code.
//
// [Ja] AppErrorCode はアプリケーションエラーを分類し、ハンドラーが HTTP ステータス
// コードに対応づけられるようにします。
type AppErrorCode int

const (
	// AppErrCodeResourceNotFound is a missing resource (404-equivalent).
	//
	// [Ja] AppErrCodeResourceNotFound はリソース未存在 (404 相当) です。
	AppErrCodeResourceNotFound AppErrorCode = iota + 1
	// AppErrCodeForbidden is insufficient permission (403-equivalent).
	//
	// [Ja] AppErrCodeForbidden は権限不足 (403 相当) です。
	AppErrCodeForbidden
	// AppErrCodeConflict is a state conflict (409-equivalent).
	//
	// [Ja] AppErrCodeConflict は状態の競合 (409 相当) です。
	AppErrCodeConflict
	// AppErrCodeInternal is a known internal failure (500-equivalent).
	//
	// [Ja] AppErrCodeInternal は想定済みの内部エラー (500 相当) です。
	AppErrCodeInternal
)

// AppError represents a known application-level failure (the SafeError pattern).
// Error() returns only the user-safe message, so the internal cause can never
// leak to the user through the error interface.
//
// [Ja] AppError は業務レベルの既知の失敗を表します (SafeError パターン)。Error() は
// ユーザー安全なメッセージのみを返すため、内部原因が error インターフェース経由で
// ユーザーに漏れることはありません。
type AppError struct {
	// Code is the error kind a handler uses to decide the status code.
	//
	// [Ja] Code はハンドラーがステータスコードを決めるために使うエラー種別です。
	Code AppErrorCode
	// UserMsg is the user-safe message. It must not contain internal details.
	//
	// [Ja] UserMsg はユーザー安全なメッセージです。内部情報を含めてはなりません。
	UserMsg string
	// Internal is the underlying error for logging; it is never shown to users.
	//
	// [Ja] Internal はログ出力用の内部エラーです。ユーザーには公開しません。
	Internal error
	// Metadata is structured-logging context such as user_id or resource_id.
	//
	// [Ja] Metadata は user_id や resource_id などの構造化ログ用コンテキストです。
	Metadata map[string]string
}

// Error returns only the user-safe message.
//
// [Ja] Error はユーザー安全なメッセージのみを返します。
func (e *AppError) Error() string { return e.UserMsg }

// Unwrap exposes the internal error to the errors.Is / errors.As chain.
//
// [Ja] Unwrap は内部エラーを errors.Is / errors.As のチェーンに公開します。
func (e *AppError) Unwrap() error { return e.Internal }

// LogString returns a detailed, log-only representation including the internal
// cause and metadata.
//
// [Ja] LogString は内部原因とメタデータを含む、ログ専用の詳細表現を返します。
func (e *AppError) LogString() string {
	return fmt.Sprintf("Code: %d | Msg: %s | Cause: %v | Meta: %v",
		e.Code, e.UserMsg, e.Internal, e.Metadata)
}

// AsValidationError extracts a *ValidationError from err, returning nil when err
// is not (and does not wrap) one.
//
// [Ja] AsValidationError は err から *ValidationError を取り出します。err がそれで
// ない (ラップもしていない) 場合は nil を返します。
func AsValidationError(err error) *ValidationError {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve
	}
	return nil
}

// AsAppError extracts an *AppError from err, returning nil when err is not (and
// does not wrap) one.
//
// [Ja] AsAppError は err から *AppError を取り出します。err がそれでない (ラップも
// していない) 場合は nil を返します。
func AsAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}
