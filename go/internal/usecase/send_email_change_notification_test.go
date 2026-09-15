package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/usecase"
)

// fakeEmailChangeNotificationSenderは最後のSend呼び出しの引数を記録し、任意で
// 指定したエラーを返す。UseCaseが何を渡したか、失敗をどう伝搬するかをテストで検証する
// ため。
type fakeEmailChangeNotificationSender struct {
	called     bool
	to         string
	newEmail   string
	locale     model.Locale
	returnErr  error
	callsCount int
}

func (f *fakeEmailChangeNotificationSender) Send(_ context.Context, to, newEmail string, locale model.Locale) error {
	f.called = true
	f.callsCount++
	f.to = to
	f.newEmail = newEmail
	f.locale = locale
	return f.returnErr
}

func TestSendEmailChangeNotificationUsecase_Execute(t *testing.T) {
	t.Parallel()

	t.Run("送信内容をsenderにそのまま渡す", func(t *testing.T) {
		t.Parallel()

		fake := &fakeEmailChangeNotificationSender{}
		uc := usecase.NewSendEmailChangeNotificationUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendEmailChangeNotificationInput{
			Email:    "old@example.dev",
			NewEmail: "new@example.dev",
			Locale:   "ja",
		})
		if err != nil {
			t.Fatalf("Execute()のエラー = %v", err)
		}

		if !fake.called {
			t.Fatal("sender.Sendが呼ばれていません")
		}
		if fake.callsCount != 1 {
			t.Errorf("Sendの呼び出し回数 = %d、期待値 = 1", fake.callsCount)
		}
		if fake.to != "old@example.dev" {
			t.Errorf("to = %q、期待値 = %q", fake.to, "old@example.dev")
		}
		if fake.newEmail != "new@example.dev" {
			t.Errorf("newEmail = %q、期待値 = %q", fake.newEmail, "new@example.dev")
		}
		if fake.locale != "ja" {
			t.Errorf("locale = %q、期待値 = %q", fake.locale, "ja")
		}
	})

	t.Run("senderの失敗をそのまま返す", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("送信失敗")
		fake := &fakeEmailChangeNotificationSender{returnErr: wantErr}
		uc := usecase.NewSendEmailChangeNotificationUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendEmailChangeNotificationInput{
			Email:    "old@example.dev",
			NewEmail: "new@example.dev",
			Locale:   "en",
		})
		if !errors.Is(err, wantErr) {
			t.Errorf("Execute()のエラー = %v、期待値 = %v", err, wantErr)
		}
	})
}
