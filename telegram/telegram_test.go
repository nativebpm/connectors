package telegram

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	_, err := NewClient("")
	assert.Error(t, err)

	client, err := NewClient("token")
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestSendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/botTEST_TOKEN/sendMessage", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var params SendMessageParams
		err = json.Unmarshal(body, &params)
		require.NoError(t, err)

		assert.Equal(t, float64(123456), params.ChatID.(float64))
		assert.Equal(t, "Hello World", params.Text)
		assert.Equal(t, "HTML", params.ParseMode)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"result": {
				"message_id": 999,
				"chat": {
					"id": 123456,
					"type": "private"
				},
				"date": 1700000000,
				"text": "Hello World"
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient("TEST_TOKEN", WithAPIURL(server.URL))
	require.NoError(t, err)

	msg, err := client.SendMessage(context.Background(), &SendMessageParams{
		ChatID:    int64(123456),
		Text:      "Hello World",
		ParseMode: "HTML",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(999), msg.MessageID)
	assert.Equal(t, "Hello World", msg.Text)
	assert.Equal(t, int64(123456), msg.Chat.ID)
}

func TestSendDocument(t *testing.T) {
	fileContent := "my streaming file content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/botTEST_TOKEN/sendDocument", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "multipart/form-data", mediaType)

		mr := multipart.NewReader(r.Body, params["boundary"])

		chatIdFound := false
		fileFound := false
		captionFound := false

		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			switch p.FormName() {
			case "chat_id":
				chatIdFound = true
				b, err := io.ReadAll(p)
				require.NoError(t, err)
				assert.Equal(t, "123456", string(b))
			case "caption":
				captionFound = true
				b, err := io.ReadAll(p)
				require.NoError(t, err)
				assert.Equal(t, "here is a file", string(b))
			case "document":
				fileFound = true
				assert.Equal(t, "test.txt", p.FileName())
				b, err := io.ReadAll(p)
				require.NoError(t, err)
				assert.Equal(t, fileContent, string(b))
			}
		}

		assert.True(t, chatIdFound)
		assert.True(t, fileFound)
		assert.True(t, captionFound)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"result": {
				"message_id": 1000,
				"chat": {
					"id": 123456,
					"type": "private"
				},
				"date": 1700000000,
				"document": {
					"file_id": "file123",
					"file_unique_id": "unique123",
					"file_name": "test.txt",
					"mime_type": "text/plain",
					"file_size": 25
				}
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient("TEST_TOKEN", WithAPIURL(server.URL))
	require.NoError(t, err)

	msg, err := client.SendDocument(context.Background(), &SendDocumentParams{
		ChatID:   int64(123456),
		Document: strings.NewReader(fileContent),
		Filename: "test.txt",
		Caption:  "here is a file",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1000), msg.MessageID)
	assert.NotNil(t, msg.Document)
	assert.Equal(t, "file123", msg.Document.FileID)
	assert.Equal(t, "test.txt", msg.Document.FileName)
}

func TestSendPhoto(t *testing.T) {
	photoContent := "fake_photo_data"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/botTEST_TOKEN/sendPhoto", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		assert.Equal(t, "multipart/form-data", mediaType)

		mr := multipart.NewReader(r.Body, params["boundary"])

		chatIdFound := false
		fileFound := false

		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)

			switch p.FormName() {
			case "chat_id":
				chatIdFound = true
				b, err := io.ReadAll(p)
				require.NoError(t, err)
				assert.Equal(t, "123456", string(b))
			case "photo":
				fileFound = true
				assert.Equal(t, "photo.jpg", p.FileName())
				b, err := io.ReadAll(p)
				require.NoError(t, err)
				assert.Equal(t, photoContent, string(b))
			}
		}

		assert.True(t, chatIdFound)
		assert.True(t, fileFound)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"result": {
				"message_id": 1001,
				"chat": {
					"id": 123456,
					"type": "private"
				},
				"date": 1700000000,
				"photo": [
					{
						"file_id": "photo123",
						"file_unique_id": "uphoto123",
						"width": 100,
						"height": 100
					}
				]
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient("TEST_TOKEN", WithAPIURL(server.URL))
	require.NoError(t, err)

	msg, err := client.SendPhoto(context.Background(), &SendPhotoParams{
		ChatID:   int64(123456),
		Photo:    strings.NewReader(photoContent),
		Filename: "photo.jpg",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1001), msg.MessageID)
	assert.NotEmpty(t, msg.Photo)
	assert.Equal(t, "photo123", msg.Photo[0].FileID)
}

func TestAPIErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"ok": false,
			"error_code": 400,
			"description": "Bad Request: chat not found"
		}`))
	}))
	defer server.Close()

	client, err := NewClient("TEST_TOKEN", WithAPIURL(server.URL))
	require.NoError(t, err)

	_, err = client.SendMessage(context.Background(), &SendMessageParams{
		ChatID: int64(99999),
		Text:   "hello",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "telegram API error (code 400): Bad Request: chat not found")
}

func TestOtherMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/botTEST_TOKEN/answerCallbackQuery" {
			_, _ = w.Write([]byte(`{"ok": true, "result": true}`))
		} else if r.URL.Path == "/botTEST_TOKEN/setWebhook" {
			_, _ = w.Write([]byte(`{"ok": true, "result": true}`))
		} else if r.URL.Path == "/botTEST_TOKEN/getUpdates" {
			_, _ = w.Write([]byte(`{"ok": true, "result": [{"update_id": 100, "message": {"message_id": 1, "chat": {"id": 2, "type": "type"}, "date": 3, "text": "hello"}}]}`))
		}
	}))
	defer server.Close()

	client, err := NewClient("TEST_TOKEN", WithAPIURL(server.URL))
	require.NoError(t, err)

	ok, err := client.AnswerCallbackQuery(context.Background(), &AnswerCallbackQueryParams{CallbackQueryID: "123"})
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = client.SetWebhook(context.Background(), &SetWebhookParams{URL: "https://example.com/webhook"})
	require.NoError(t, err)
	assert.True(t, ok)

	updates, err := client.GetUpdates(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, int64(100), updates[0].UpdateID)
	assert.Equal(t, "hello", updates[0].Message.Text)
}

func TestFluentBuilders(t *testing.T) {
	photoContent := "fake_photo_data"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/botTEST_TOKEN/sendMessage" {
			_, _ = w.Write([]byte(`{"ok": true, "result": {"message_id": 1, "chat": {"id": 123456, "type": "private"}, "date": 3, "text": "hello"}}`))
		} else if r.URL.Path == "/botTEST_TOKEN/sendDocument" {
			_, _ = w.Write([]byte(`{"ok": true, "result": {"message_id": 2, "chat": {"id": 123456, "type": "private"}, "date": 3, "document": {"file_id": "doc123"}}}`))
		} else if r.URL.Path == "/botTEST_TOKEN/sendPhoto" {
			_, _ = w.Write([]byte(`{"ok": true, "result": {"message_id": 3, "chat": {"id": 123456, "type": "private"}, "date": 3, "photo": [{"file_id": "photo123"}]}}`))
		}
	}))
	defer server.Close()

	client, err := NewClient("TEST_TOKEN", WithAPIURL(server.URL))
	require.NoError(t, err)

	// Test Message Builder
	msg, err := client.NewMessage(int64(123456), "hello").
		ParseMode("HTML").
		DisableWebPagePreview(true).
		DisableNotification(true).
		ReplyTo(999).
		Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), msg.MessageID)
	assert.Equal(t, "hello", msg.Text)

	// Test Document Builder
	docMsg, err := client.NewDocument(int64(123456), strings.NewReader("doc"), "doc.txt").
		Caption("my doc").
		ParseMode("HTML").
		DisableNotification(true).
		ReplyTo(999).
		ReplyMarkup(nil).
		Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), docMsg.MessageID)
	assert.Equal(t, "doc123", docMsg.Document.FileID)

	// Test Photo Builder
	photoMsg, err := client.NewPhoto(int64(123456), strings.NewReader(photoContent), "photo.jpg").
		Caption("my photo").
		ParseMode("HTML").
		DisableNotification(true).
		ReplyTo(999).
		ReplyMarkup(nil).
		Send(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(3), photoMsg.MessageID)
	assert.Equal(t, "photo123", photoMsg.Photo[0].FileID)
}

func TestStartPolling(t *testing.T) {
	called := make(chan Update, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/botTEST_TOKEN/getUpdates", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"result": [
				{
					"update_id": 999,
					"message": {
						"message_id": 1,
						"chat": {
							"id": 123456,
							"type": "private"
						},
						"date": 1700000000,
						"text": "test message"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client, err := NewClient("TEST_TOKEN", WithAPIURL(server.URL))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := client.StartPolling(ctx, func(ctx context.Context, update Update) {
			called <- update
			cancel() // Stop polling after receiving the first update
		})
		assert.ErrorIs(t, err, context.Canceled)
	}()

	select {
	case update := <-called:
		assert.Equal(t, int64(999), update.UpdateID)
		assert.Equal(t, "test message", update.Message.Text)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for polling update callback")
	}
}

func TestKeyboardBuilders(t *testing.T) {
	// 1. Test InlineKeyboardBuilder
	ik := NewInlineKeyboard().
		AddButton("btn1", "data1").
		AddURLButton("btn2", "https://google.com").
		AddButtonRow("btn3", "data3").
		AddURLButtonRow("btn4", "https://yahoo.com").
		Build()

	require.Len(t, ik.InlineKeyboard, 3)
	// Row 1 (added using AddButton and AddURLButton)
	require.Len(t, ik.InlineKeyboard[0], 2)
	assert.Equal(t, "btn1", ik.InlineKeyboard[0][0].Text)
	assert.Equal(t, "data1", *ik.InlineKeyboard[0][0].CallbackData)
	assert.Nil(t, ik.InlineKeyboard[0][0].URL)

	assert.Equal(t, "btn2", ik.InlineKeyboard[0][1].Text)
	assert.Equal(t, "https://google.com", *ik.InlineKeyboard[0][1].URL)
	assert.Nil(t, ik.InlineKeyboard[0][1].CallbackData)

	// Row 2 (added using AddButtonRow)
	require.Len(t, ik.InlineKeyboard[1], 1)
	assert.Equal(t, "btn3", ik.InlineKeyboard[1][0].Text)

	// Row 3 (added using AddURLButtonRow)
	require.Len(t, ik.InlineKeyboard[2], 1)
	assert.Equal(t, "btn4", ik.InlineKeyboard[2][0].Text)

	// 2. Test ReplyKeyboardBuilder
	rk := NewReplyKeyboard().
		AddButton("reply1").
		AddButton("reply2").
		AddButtonRow("reply3").
		ResizeKeyboard(true).
		OneTimeKeyboard(true).
		Build()

	require.Len(t, rk.Keyboard, 2)
	require.Len(t, rk.Keyboard[0], 2)
	assert.Equal(t, "reply1", rk.Keyboard[0][0].Text)
	assert.Equal(t, "reply2", rk.Keyboard[0][1].Text)

	require.Len(t, rk.Keyboard[1], 1)
	assert.Equal(t, "reply3", rk.Keyboard[1][0].Text)

	assert.True(t, rk.ResizeKeyboard)
	assert.True(t, rk.OneTimeKeyboard)
}
