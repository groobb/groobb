package health

import (
	"encoding/json"
	"net/http"
)

// Showはサーバーが稼働中であることを示すJSONを返します。
//
// エンコード失敗時にも500を返せるよう、ヘッダーを書く前にボディを
// marshalする。WriteHeaderを呼んだ後はステータスコードが確定し、
// 後続のhttp.Errorは無効になるため。
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
