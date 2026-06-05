package services

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTelegramAPISendMessageIncludesMessageThreadID(t *testing.T) {
	var payload map[string]interface{}
	api := &TelegramAPI{
		BotToken: "token",
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				bytes, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if err := json.Unmarshal(bytes, &payload); err != nil {
					t.Fatalf("Unmarshal() error = %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}

	if err := api.SendMessage("-1003963321501", TelegramMessage{
		Text:            "hello",
		ParseMode:       "none",
		MessageThreadID: 4,
	}); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	if payload["chat_id"] != "-1003963321501" {
		t.Fatalf("chat_id = %#v, want -1003963321501", payload["chat_id"])
	}
	if payload["message_thread_id"] != float64(4) {
		t.Fatalf("message_thread_id = %#v, want 4", payload["message_thread_id"])
	}
	if _, ok := payload["parse_mode"]; ok {
		t.Fatalf("parse_mode present for ParseMode=none: %#v", payload["parse_mode"])
	}
}
