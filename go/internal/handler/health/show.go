package health

import (
	"encoding/json"
	"net/http"
)

// Show responds with a JSON body indicating the server is alive.
//
// The body is marshaled before any header is written so that an encoding
// failure can still be reported as a 500. Once WriteHeader has been called the
// status code is fixed and a later http.Error would be a no-op.
//
// [Ja] Show はサーバーが稼働中であることを示す JSON を返します。
//
// エンコード失敗時にも 500 を返せるよう、ヘッダーを書く前にボディを
// marshal する。WriteHeader を呼んだ後はステータスコードが確定し、
// 後続の http.Error は無効になるため。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	body, err := json.Marshal(map[string]string{
		"status": "ok",
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
