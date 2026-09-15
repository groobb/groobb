package middleware

import (
	"net/http"
	"strings"
)

// MethodOverrideは、_methodフォームフィールドの値が状態変更メソッド
// (PATCH / PUT / DELETE) を指すとき、POSTリクエストのメソッドをその値へ書き換えます。
// HTMLフォームはGETとPOSTしか送れないため、hiddenな _methodフィールドを
// 持たせることでフォームからPATCH / DELETEのルートを動かせます。POST以外の
// リクエストや、未知・非対応の _method値はそのままにします。
func MethodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// ParseFormは解析済みbodyをリクエストにキャッシュするため、この
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
