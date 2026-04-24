package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gatecli/gatecli"
	flags "github.com/jessevdk/go-flags"
)

func TestParseTelegramConfigData(t *testing.T) {
	cfg, err := parseTelegramConfigData([]byte(`{"botToken":"abc","pollTimeout":"45s","requestTimeout":"90s","allowedUpdates":["message"]}`))
	if err != nil {
		t.Fatalf("parseTelegramConfigData() error = %v", err)
	}
	if cfg.BotToken != "abc" {
		t.Fatalf("BotToken = %q", cfg.BotToken)
	}
	if cfg.PollTimeout != 45*time.Second {
		t.Fatalf("PollTimeout = %v", cfg.PollTimeout)
	}
	if cfg.RequestTimeout != 90*time.Second {
		t.Fatalf("RequestTimeout = %v", cfg.RequestTimeout)
	}
	if len(cfg.AllowedUpdates) != 1 || cfg.AllowedUpdates[0] != "message" {
		t.Fatalf("AllowedUpdates = %#v", cfg.AllowedUpdates)
	}
}

func TestParseTelegramConfigDataAllowsEmptyAllowedUpdates(t *testing.T) {
	cfg, err := parseTelegramConfigData([]byte(`{"botToken":"abc","allowedUpdates":[]}`))
	if err != nil {
		t.Fatalf("parseTelegramConfigData() error = %v", err)
	}
	if cfg.AllowedUpdates == nil {
		t.Fatalf("AllowedUpdates should preserve explicit empty slice")
	}
	if len(cfg.AllowedUpdates) != 0 {
		t.Fatalf("AllowedUpdates = %#v", cfg.AllowedUpdates)
	}
}

func TestRenderTelegramHTML(t *testing.T) {
	items := []gatecli.MessageItem{
		gatecli.TextItem("你好 "),
		{Type: "at", Fields: map[string]string{"id": "123456", "name": "Alice"}},
		gatecli.TextItem(" & welcome"),
	}
	got, err := renderTelegramHTML(items)
	if err != nil {
		t.Fatalf("renderTelegramHTML() error = %v", err)
	}
	want := "你好 <a href=\"tg://user?id=123456\">Alice</a> &amp; welcome"
	if got != want {
		t.Fatalf("renderTelegramHTML() = %q, want %q", got, want)
	}
}

func TestWriteTelegramConfigValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := writeTelegramConfigValue(path, "botToken", "abc123"); err != nil {
		t.Fatalf("writeTelegramConfigValue() error = %v", err)
	}
	if err := writeTelegramConfigValue(path, "apiBase", "https://api.telegram.org"); err != nil {
		t.Fatalf("writeTelegramConfigValue() second write error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got["botToken"] != "abc123" {
		t.Fatalf("botToken = %#v", got["botToken"])
	}
	if got["apiBase"] != "https://api.telegram.org" {
		t.Fatalf("apiBase = %#v", got["apiBase"])
	}
}

func TestAuthCommandAcceptsPositionalToken(t *testing.T) {
	cmd := &authCommand{}
	parser := flags.NewParser(cmd, flags.HelpFlag|flags.PassDoubleDash)
	if _, err := parser.ParseArgs([]string{"abc123"}); err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if cmd.Args.Token != "abc123" {
		t.Fatalf("token = %q", cmd.Args.Token)
	}
}

func TestDefaultMediaOutputPath(t *testing.T) {
	base := "/tmp/media/1234"
	got := defaultMediaOutputPath(base, telegramMediaMetadata{Format: "jpg"})
	if got != "/tmp/media/1234.jpg" {
		t.Fatalf("defaultMediaOutputPath() = %q", got)
	}
	got = defaultMediaOutputPath(base, telegramMediaMetadata{FileName: "voice.ogg", Format: "oga"})
	if got != "/tmp/media/1234.ogg" {
		t.Fatalf("defaultMediaOutputPath() filename ext = %q", got)
	}
}

func TestMediaItemUsesResourceID(t *testing.T) {
	item := gatecli.MessageItem{Type: "image", Fields: map[string]string{"resource_id": "resource-123", "format": "jpg"}}
	if got := item.Fields["resource_id"]; got != "resource-123" {
		t.Fatalf("resource_id = %q", got)
	}
	if _, ok := item.Fields["id"]; ok {
		t.Fatalf("unexpected id field: %#v", item.Fields)
	}
}

func TestResolveMediaSourceSupportsResourceID(t *testing.T) {
	service := &TelegramService{}
	item := gatecli.MessageItem{Type: "image", Fields: map[string]string{"resource_id": "media-123"}}
	if got := item.Get("resource_id"); got != "media-123" {
		t.Fatalf("resource_id = %q", got)
	}
	_, err := service.resolveMediaSource(context.Background(), item)
	if err == nil {
		t.Fatalf("resolveMediaSource() expected error without app")
	}
	if err.Error() != "app is not initialized" {
		t.Fatalf("resolveMediaSource() error = %v", err)
	}
}

func TestBuildReplyKeyboardMarkup(t *testing.T) {
	markup, err := buildReplyKeyboardMarkup([]string{"A|B", "C"}, true, true, false, "pick one", false)
	if err != nil {
		t.Fatalf("buildReplyKeyboardMarkup() error = %v", err)
	}
	if len(markup.Keyboard) != 2 {
		t.Fatalf("rows = %d", len(markup.Keyboard))
	}
	if markup.Keyboard[0][0].Text != "A" || markup.Keyboard[0][1].Text != "B" {
		t.Fatalf("first row = %#v", markup.Keyboard[0])
	}
	if !markup.ResizeKeyboard || !markup.OneTimeKeyboard {
		t.Fatalf("flags = %#v", markup)
	}
	if markup.InputFieldPlaceholder != "pick one" {
		t.Fatalf("placeholder = %q", markup.InputFieldPlaceholder)
	}
}

func TestBuildInlineKeyboardMarkup(t *testing.T) {
	markup, err := buildInlineKeyboardMarkup([]string{"Docs=https://example.com|Ping=data:ping", "Raw=callback"})
	if err != nil {
		t.Fatalf("buildInlineKeyboardMarkup() error = %v", err)
	}
	if len(markup.InlineKeyboard) != 2 {
		t.Fatalf("rows = %d", len(markup.InlineKeyboard))
	}
	if markup.InlineKeyboard[0][0].URL != "https://example.com" {
		t.Fatalf("first button url = %#v", markup.InlineKeyboard[0][0])
	}
	if markup.InlineKeyboard[0][1].CallbackData != "ping" {
		t.Fatalf("second button callback = %#v", markup.InlineKeyboard[0][1])
	}
	if markup.InlineKeyboard[1][0].CallbackData != "callback" {
		t.Fatalf("third button callback = %#v", markup.InlineKeyboard[1][0])
	}
}

func TestBuildInlineKeyboardMarkupRejectsInvalidButton(t *testing.T) {
	if _, err := buildInlineKeyboardMarkup([]string{"Broken"}); err == nil {
		t.Fatalf("buildInlineKeyboardMarkup() expected error")
	}
}

func TestResolveCommandTarget(t *testing.T) {
	service := &TelegramService{}
	got, err := resolveCommandTarget(service, "123", "", false)
	if err != nil {
		t.Fatalf("resolveCommandTarget() direct error = %v", err)
	}
	if got != "123" {
		t.Fatalf("resolveCommandTarget() direct = %q", got)
	}
	if _, err := resolveCommandTarget(service, "", "", false); err == nil {
		t.Fatalf("resolveCommandTarget() expected missing target error")
	}
}

func TestRawUpdateType(t *testing.T) {
	got := rawUpdateType(json.RawMessage(`{"update_id":1,"callback_query":{"id":"abc"}}`))
	if got != "callback_query" {
		t.Fatalf("rawUpdateType() = %q", got)
	}
}

func TestUpdateEnvelopeFromRawAllEvents(t *testing.T) {
	service := &TelegramService{}
	envelope, ok, err := service.updateEnvelopeFromRaw(
		context.Background(),
		telegramUpdate{UpdateID: 42},
		json.RawMessage(`{"update_id":42,"callback_query":{"id":"abc"}}`),
		true,
	)
	if err != nil {
		t.Fatalf("updateEnvelopeFromRaw() error = %v", err)
	}
	if !ok {
		t.Fatalf("updateEnvelopeFromRaw() expected event envelope")
	}
	if envelope.ID != "42" {
		t.Fatalf("id = %q", envelope.ID)
	}
	if envelope.User != "telegram:callback_query" {
		t.Fatalf("user = %q", envelope.User)
	}
	if len(envelope.Items) != 1 || envelope.Items[0].Type != "text" {
		t.Fatalf("items = %#v", envelope.Items)
	}
}

func TestSendMethodForType(t *testing.T) {
	method, field, err := sendMethodForType("image")
	if err != nil {
		t.Fatalf("sendMethodForType() error = %v", err)
	}
	if method != "sendPhoto" || field != "photo" {
		t.Fatalf("sendMethodForType() = (%q, %q)", method, field)
	}
}
