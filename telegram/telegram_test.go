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
