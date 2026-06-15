package middleware

import (
	"net/http"
	"strings"
)

// MethodOverride rewrites a POST request's method to the value of the _method
// form field when that value names a state-changing method (PATCH / PUT /
// DELETE). HTML forms can only issue GET and POST, so this lets a form drive a
// PATCH / DELETE route by carrying a hidden _method field. Requests that are not
// POST, and unknown or unsupported _method values, are left untouched.
//
// [Ja] MethodOverride は、_method フォームフィールドの値が状態変更メソッド
// (PATCH / PUT / DELETE) を指すとき、POST リクエストのメソッドをその値へ書き換えます。
// HTML フォームは GET と POST しか送れないため、hidden な _method フィールドを
// 持たせることでフォームから PATCH / DELETE のルートを動かせます。POST 以外の
// リクエストや、未知・非対応の _method 値はそのままにします。
func MethodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// ParseForm caches the parsed body on the request, so the downstream
			// handler can still read other form values after this lookup.
			//
			// [Ja] ParseForm は解析済み body をリクエストにキャッシュするため、この
			// 読み取り後も後続ハンドラーは他のフォーム値を読める。
			if err := r.ParseForm(); err == nil {
				method := strings.ToUpper(r.PostFormValue("_method"))
				switch method {
				case http.MethodPut, http.MethodPatch, http.MethodDelete:
					r.Method = method
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
