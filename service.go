package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gatecli/gatecli"
	flags "github.com/jessevdk/go-flags"
)

type TelegramService struct {
	app    *gatecli.App
	client *http.Client
}

func NewTelegramService() *TelegramService {
	return &TelegramService{client: http.DefaultClient}
}

func (s *TelegramService) ServiceName() string {
	return "telegram"
}

func (s *TelegramService) HookNames() []string {
	return []string{"telegram", "handlers", "hooks"}
}

func (s *TelegramService) RegisterCommands(app *gatecli.App, parser *flags.Parser) error {
	s.app = app
	if _, err := app.AddCommand("auth", "write bot token", "Write the Telegram bot token into the config file.", &authCommand{service: s}); err != nil {
		return err
	}
	if _, err := app.AddCommand("keyboard", "keyboard operations", "Set or remove Telegram reply and inline keyboards.", &keyboardCommand{service: s}); err != nil {
		return err
	}
	if _, err := app.AddCommand("me", "show bot profile", "Show the current Telegram bot profile.", &meCommand{service: s}); err != nil {
		return err
	}
	if _, err := app.AddCommand("chat", "chat operations", "Inspect Telegram chat information.", &chatCommand{service: s}); err != nil {
		return err
	}
	if _, err := app.AddCommand("history", "show history", "Show locally stored message history.", &historyCommand{service: s}); err != nil {
		return err
	}
	if _, err := app.AddCommand("media", "media operations", "Download and inspect stored media metadata.", &mediaCommand{service: s}); err != nil {
		return err
	}
	return nil
}

func (s *TelegramService) SendMessage(ctx context.Context, req gatecli.SendRequest) (gatecli.SendResult, error) {
	cfg, err := s.loadTelegramConfig()
	if err != nil {
		return gatecli.SendResult{}, err
	}

	var lastMessageID string
	textItems := make([]gatecli.MessageItem, 0)
	flushText := func() error {
		if len(textItems) == 0 {
			return nil
		}
		rendered, err := renderTelegramHTML(textItems)
		if err != nil {
			return err
		}
		if strings.TrimSpace(stripHTMLText(rendered)) == "" {
			textItems = textItems[:0]
			return nil
		}
		sent, err := s.sendRenderedTextWithReplyMarkup(ctx, cfg, req.User, rendered, nil, false, nil)
		if err != nil {
			return err
		}
		lastMessageID = strconv.FormatInt(sent.MessageID, 10)
		textItems = textItems[:0]
		return nil
	}

	for _, item := range req.Items {
		if isTextLike(item) {
			textItems = append(textItems, item)
			continue
		}

		captionItems := append([]gatecli.MessageItem(nil), textItems...)
		textItems = textItems[:0]

		method, field, err := sendMethodForType(item.Type)
		if err != nil {
			return gatecli.SendResult{}, err
		}
		source, err := s.resolveMediaSource(ctx, item)
		if err != nil {
			return gatecli.SendResult{}, err
		}
		params := map[string]any{
			"chat_id": req.User,
			field:     source,
		}
		if len(captionItems) > 0 {
			caption, err := renderTelegramHTML(captionItems)
			if err != nil {
				return gatecli.SendResult{}, err
			}
			if strings.TrimSpace(stripHTMLText(caption)) != "" {
				params["caption"] = caption
				params["parse_mode"] = "HTML"
			}
		}
		var sent telegramMessage
		if err := s.callTelegram(ctx, cfg, method, params, &sent); err != nil {
			return gatecli.SendResult{}, err
		}
		lastMessageID = strconv.FormatInt(sent.MessageID, 10)
	}

	if err := flushText(); err != nil {
		return gatecli.SendResult{}, err
	}
	return gatecli.SendResult{MessageID: lastMessageID}, nil
}

func (s *TelegramService) Watch(ctx context.Context, opts gatecli.WatchOptions, handler func(context.Context, gatecli.WatchEvent) error) error {
	cfg, err := s.loadTelegramConfig()
	if err != nil {
		return err
	}

	var offset int64
	pollSeconds := int(cfg.PollTimeout / time.Second)
	if pollSeconds <= 0 {
		pollSeconds = 1
	}
	allEvents := cfg.AllowedUpdates != nil && len(cfg.AllowedUpdates) == 0

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		params := map[string]any{
			"timeout": pollSeconds,
		}
		if cfg.AllowedUpdates != nil {
			params["allowed_updates"] = cfg.AllowedUpdates
		}
		if offset > 0 {
			params["offset"] = offset
		}

		var rawUpdates []json.RawMessage
		if err := s.callTelegram(ctx, cfg, "getUpdates", params, &rawUpdates); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		for _, rawUpdate := range rawUpdates {
			var update telegramUpdate
			if err := json.Unmarshal(rawUpdate, &update); err != nil {
				return fmt.Errorf("decode telegram update: %w", err)
			}
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			envelope, ok, err := s.updateEnvelopeFromRaw(ctx, update, rawUpdate, allEvents)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if strings.TrimSpace(opts.User) != "" && envelope.User != strings.TrimSpace(opts.User) {
				continue
			}
			if err := handler(ctx, gatecli.WatchEvent{Message: envelope}); err != nil {
				return err
			}
		}
	}
}

func (s *TelegramService) callTelegram(ctx context.Context, cfg TelegramConfig, method string, params any, result any) error {
	callCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()

	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/bot%s/%s", cfg.APIBase, cfg.BotToken, method)
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var envelope telegramAPIResponse
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	if !envelope.OK {
		if envelope.Description == "" {
			return fmt.Errorf("telegram %s failed with code %d", method, envelope.ErrorCode)
		}
		return fmt.Errorf("telegram %s failed: %s", method, envelope.Description)
	}
	if result == nil || len(envelope.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode %s result: %w", method, err)
	}
	return nil
}

func (s *TelegramService) updateEnvelopeFromRaw(ctx context.Context, update telegramUpdate, rawUpdate json.RawMessage, allEvents bool) (gatecli.MessageEnvelope, bool, error) {
	if message := pickUpdateMessage(update); message != nil {
		items, err := s.messageItemsFromTelegram(ctx, message)
		if err != nil {
			return gatecli.MessageEnvelope{}, false, err
		}
		if len(items) == 0 {
			return gatecli.MessageEnvelope{}, false, nil
		}
		return gatecli.MessageEnvelope{
			ID:       strconv.FormatInt(message.MessageID, 10),
			DateTime: time.Unix(message.Date, 0).UTC(),
			User:     strconv.FormatInt(message.Chat.ID, 10),
			Items:    items,
		}, true, nil
	}
	if !allEvents {
		return gatecli.MessageEnvelope{}, false, nil
	}
	eventType := rawUpdateType(rawUpdate)
	if eventType == "" {
		eventType = "update"
	}
	return gatecli.MessageEnvelope{
		ID:       strconv.FormatInt(update.UpdateID, 10),
		DateTime: time.Now().UTC(),
		User:     "telegram:" + eventType,
		Items:    []gatecli.MessageItem{gatecli.TextItem(string(rawUpdate))},
	}, true, nil
}

func rawUpdateType(rawUpdate json.RawMessage) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawUpdate, &payload); err != nil {
		return ""
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		if key == "update_id" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return keys[0]
}

func pickUpdateMessage(update telegramUpdate) *telegramMessage {
	switch {
	case update.Message != nil:
		return update.Message
	case update.EditedMessage != nil:
		return update.EditedMessage
	case update.ChannelPost != nil:
		return update.ChannelPost
	case update.EditedChannelPost != nil:
		return update.EditedChannelPost
	case update.BusinessMessage != nil:
		return update.BusinessMessage
	case update.EditedBusinessMsg != nil:
		return update.EditedBusinessMsg
	default:
		return nil
	}
}

func (s *TelegramService) messageItemsFromTelegram(ctx context.Context, message *telegramMessage) ([]gatecli.MessageItem, error) {
	items := make([]gatecli.MessageItem, 0)
	if strings.TrimSpace(message.Text) != "" {
		items = append(items, gatecli.TextItem(message.Text))
	}
	if strings.TrimSpace(message.Caption) != "" {
		items = append(items, gatecli.TextItem(message.Caption))
	}

	if len(message.Photo) > 0 {
		photo := message.Photo[len(message.Photo)-1]
		item, err := s.storeMediaItem(ctx, telegramMediaMetadata{
			Type:         "image",
			FileID:       photo.FileID,
			FileUniqueID: photo.FileUniqueID,
			FileSize:     photo.FileSize,
			Format:       "jpg",
			ChatID:       message.Chat.ID,
			MessageID:    message.MessageID,
			Date:         message.Date,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if message.Audio != nil {
		item, err := s.storeMediaItem(ctx, telegramMediaMetadata{
			Type:         "audio",
			FileID:       message.Audio.FileID,
			FileUniqueID: message.Audio.FileUniqueID,
			FileName:     message.Audio.FileName,
			MimeType:     message.Audio.MimeType,
			FileSize:     message.Audio.FileSize,
			Format:       fileFormat(message.Audio.FileName, message.Audio.MimeType, "audio"),
			ChatID:       message.Chat.ID,
			MessageID:    message.MessageID,
			Date:         message.Date,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if message.Document != nil {
		item, err := s.storeMediaItem(ctx, telegramMediaMetadata{
			Type:         "document",
			FileID:       message.Document.FileID,
			FileUniqueID: message.Document.FileUniqueID,
			FileName:     message.Document.FileName,
			MimeType:     message.Document.MimeType,
			FileSize:     message.Document.FileSize,
			Format:       fileFormat(message.Document.FileName, message.Document.MimeType, "document"),
			ChatID:       message.Chat.ID,
			MessageID:    message.MessageID,
			Date:         message.Date,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if message.Video != nil {
		item, err := s.storeMediaItem(ctx, telegramMediaMetadata{
			Type:         "video",
			FileID:       message.Video.FileID,
			FileUniqueID: message.Video.FileUniqueID,
			FileName:     message.Video.FileName,
			MimeType:     message.Video.MimeType,
			FileSize:     message.Video.FileSize,
			Format:       fileFormat(message.Video.FileName, message.Video.MimeType, "video"),
			ChatID:       message.Chat.ID,
			MessageID:    message.MessageID,
			Date:         message.Date,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if message.Voice != nil {
		item, err := s.storeMediaItem(ctx, telegramMediaMetadata{
			Type:         "voice",
			FileID:       message.Voice.FileID,
			FileUniqueID: message.Voice.FileUniqueID,
			MimeType:     message.Voice.MimeType,
			FileSize:     message.Voice.FileSize,
			Format:       fileFormat("", message.Voice.MimeType, "voice"),
			ChatID:       message.Chat.ID,
			MessageID:    message.MessageID,
			Date:         message.Date,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if message.Sticker != nil {
		item, err := s.storeMediaItem(ctx, telegramMediaMetadata{
			Type:         "sticker",
			FileID:       message.Sticker.FileID,
			FileUniqueID: message.Sticker.FileUniqueID,
			FileSize:     message.Sticker.FileSize,
			Format:       stickerFormat(message.Sticker),
			ChatID:       message.Chat.ID,
			MessageID:    message.MessageID,
			Date:         message.Date,
			Emoji:        message.Sticker.Emoji,
		})
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *TelegramService) storeMediaItem(ctx context.Context, metadata telegramMediaMetadata) (gatecli.MessageItem, error) {
	fields := map[string]string{}
	if metadata.Format != "" {
		fields["format"] = metadata.Format
	}
	if metadata.FileName != "" {
		fields["name"] = metadata.FileName
	}
	if metadata.Emoji != "" {
		fields["emoji"] = metadata.Emoji
	}
	if s.app == nil {
		fields["file_id"] = metadata.FileID
		return gatecli.MessageItem{Type: metadata.Type, Fields: fields}, nil
	}
	stored, err := s.app.CreateMedia(ctx, metadata)
	if err != nil {
		return gatecli.MessageItem{}, err
	}
	fields["resource_id"] = stored.ID
	return gatecli.MessageItem{Type: metadata.Type, Fields: fields}, nil
}

func (s *TelegramService) resolveMediaSource(ctx context.Context, item gatecli.MessageItem) (string, error) {
	for _, key := range []string{"file_id", "fileId", "url"} {
		if value := strings.TrimSpace(item.Get(key)); value != "" {
			return value, nil
		}
	}
	mediaID := strings.TrimSpace(item.Get("resource_id"))
	if mediaID == "" {
		return "", fmt.Errorf("message item %q requires one of resource_id, file_id, fileId or url", item.Type)
	}
	if s.app == nil {
		return "", fmt.Errorf("app is not initialized")
	}
	stored, err := s.app.GetMetadataFromID(ctx, mediaID)
	if err != nil {
		return "", err
	}
	var metadata telegramMediaMetadata
	if err := json.Unmarshal(stored.Metadata, &metadata); err != nil {
		return "", err
	}
	if strings.TrimSpace(metadata.FileID) != "" {
		return strings.TrimSpace(metadata.FileID), nil
	}
	return "", fmt.Errorf("media %s does not contain a reusable Telegram file_id", mediaID)
}

func sendMethodForType(itemType string) (string, string, error) {
	switch itemType {
	case "image":
		return "sendPhoto", "photo", nil
	case "audio":
		return "sendAudio", "audio", nil
	case "document":
		return "sendDocument", "document", nil
	case "video":
		return "sendVideo", "video", nil
	case "voice":
		return "sendVoice", "voice", nil
	case "sticker":
		return "sendSticker", "sticker", nil
	case "animation":
		return "sendAnimation", "animation", nil
	default:
		return "", "", fmt.Errorf("unsupported telegram message item type %q", itemType)
	}
}

func isTextLike(item gatecli.MessageItem) bool {
	return item.Type == "" || item.Type == "text" || item.Type == "at"
}

func renderTelegramHTML(items []gatecli.MessageItem) (string, error) {
	var builder strings.Builder
	for _, item := range items {
		switch item.Type {
		case "", "text":
			builder.WriteString(html.EscapeString(item.Get("content")))
		case "at":
			if id := strings.TrimSpace(item.Get("id")); id != "" {
				label := strings.TrimSpace(item.Get("name"))
				if label == "" {
					label = id
				}
				builder.WriteString(`<a href="tg://user?id=`)
				builder.WriteString(html.EscapeString(id))
				builder.WriteString(`">`)
				builder.WriteString(html.EscapeString(label))
				builder.WriteString(`</a>`)
				continue
			}
			if username := strings.TrimSpace(item.Get("username")); username != "" {
				if !strings.HasPrefix(username, "@") {
					username = "@" + username
				}
				builder.WriteString(html.EscapeString(username))
				continue
			}
			return "", fmt.Errorf("telegram at item requires id or username")
		default:
			return "", fmt.Errorf("telegram HTML renderer does not support item type %q", item.Type)
		}
	}
	return builder.String(), nil
}

func stripHTMLText(input string) string {
	replacer := strings.NewReplacer("<br>", "", "<", "", ">", "")
	return replacer.Replace(input)
}

func fileFormat(name, mimeType, fallback string) string {
	if ext := strings.TrimPrefix(filepath.Ext(name), "."); ext != "" {
		return strings.ToLower(ext)
	}
	if slash := strings.LastIndexByte(mimeType, '/'); slash >= 0 && slash < len(mimeType)-1 {
		format := mimeType[slash+1:]
		format = strings.TrimSpace(strings.TrimSuffix(format, ";"))
		if format != "" {
			return strings.ToLower(format)
		}
	}
	return fallback
}

func stickerFormat(sticker *telegramSticker) string {
	if sticker == nil {
		return "sticker"
	}
	if sticker.IsAnimated {
		return "tgs"
	}
	if sticker.IsVideo {
		return "webm"
	}
	return "webp"
}

func (s *TelegramService) DownloadMedia(ctx context.Context, mediaID string, outputPath string) (string, error) {
	if s.app == nil {
		return "", fmt.Errorf("app is not initialized")
	}
	stored, err := s.app.GetMetadataFromID(ctx, mediaID)
	if err != nil {
		return "", err
	}
	if stored.Downloaded && strings.TrimSpace(stored.FilePath) != "" {
		if _, err := os.Stat(stored.FilePath); err == nil {
			return stored.FilePath, nil
		}
	}

	var metadata telegramMediaMetadata
	if err := json.Unmarshal(stored.Metadata, &metadata); err != nil {
		return "", fmt.Errorf("decode metadata for %s: %w", mediaID, err)
	}
	if strings.TrimSpace(metadata.FileID) == "" {
		return "", fmt.Errorf("media %s does not contain telegram file_id", mediaID)
	}
	cfg, err := s.loadTelegramConfig()
	if err != nil {
		return "", err
	}
	var file telegramFile
	if err := s.callTelegram(ctx, cfg, "getFile", map[string]any{"file_id": metadata.FileID}, &file); err != nil {
		return "", err
	}
	if strings.TrimSpace(file.FilePath) == "" {
		return "", fmt.Errorf("telegram getFile returned empty file_path for %s", mediaID)
	}

	path, err := s.prepareDownloadPath(mediaID, outputPath, metadata)
	if err != nil {
		return "", err
	}
	if err := s.downloadTelegramFile(ctx, cfg, file.FilePath, path); err != nil {
		return "", err
	}
	if err := s.app.MarkMediaDownloaded(ctx, mediaID, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *TelegramService) GetMe(ctx context.Context) (telegramUser, error) {
	cfg, err := s.loadTelegramConfig()
	if err != nil {
		return telegramUser{}, err
	}
	var user telegramUser
	if err := s.callTelegram(ctx, cfg, "getMe", map[string]any{}, &user); err != nil {
		return telegramUser{}, err
	}
	return user, nil
}

func (s *TelegramService) SendTextWithReplyMarkup(ctx context.Context, chatID string, text string, replyMarkup any) (telegramSentMessage, error) {
	cfg, err := s.loadTelegramConfig()
	if err != nil {
		return telegramSentMessage{}, err
	}
	rendered := html.EscapeString(text)
	items := []gatecli.MessageItem{gatecli.TextItem(text)}
	return s.sendRenderedTextWithReplyMarkup(ctx, cfg, chatID, rendered, replyMarkup, true, items)
}

func (s *TelegramService) sendRenderedTextWithReplyMarkup(ctx context.Context, cfg TelegramConfig, chatID string, renderedText string, replyMarkup any, storeLocal bool, items []gatecli.MessageItem) (telegramSentMessage, error) {
	params := map[string]any{
		"chat_id":    chatID,
		"text":       renderedText,
		"parse_mode": "HTML",
	}
	if replyMarkup != nil {
		params["reply_markup"] = replyMarkup
	}
	var sent telegramSentMessage
	if err := s.callTelegram(ctx, cfg, "sendMessage", params, &sent); err != nil {
		return telegramSentMessage{}, err
	}
	if storeLocal && s.app != nil {
		if _, err := s.app.Storage().StoreMessage(ctx, chatID, items); err != nil {
			return telegramSentMessage{}, err
		}
	}
	return sent, nil
}

func (s *TelegramService) GetChat(ctx context.Context, chatID string) (telegramChat, error) {
	cfg, err := s.loadTelegramConfig()
	if err != nil {
		return telegramChat{}, err
	}
	var chat telegramChat
	if err := s.callTelegram(ctx, cfg, "getChat", map[string]any{"chat_id": strings.TrimSpace(chatID)}, &chat); err != nil {
		return telegramChat{}, err
	}
	return chat, nil
}

func (s *TelegramService) GetUserProfilePhotos(ctx context.Context, userID string) (telegramUserProfilePhotos, error) {
	cfg, err := s.loadTelegramConfig()
	if err != nil {
		return telegramUserProfilePhotos{}, err
	}
	var photos telegramUserProfilePhotos
	if err := s.callTelegram(ctx, cfg, "getUserProfilePhotos", map[string]any{"user_id": strings.TrimSpace(userID)}, &photos); err != nil {
		return telegramUserProfilePhotos{}, err
	}
	return photos, nil
}

func (s *TelegramService) prepareDownloadPath(mediaID, outputPath string, metadata telegramMediaMetadata) (string, error) {
	if strings.TrimSpace(outputPath) == "" {
		path, err := s.app.PrepareMediaFilePath(mediaID)
		if err != nil {
			return "", err
		}
		return defaultMediaOutputPath(path, metadata), nil
	}
	path, err := expandUserPath(outputPath)
	if err != nil {
		return "", err
	}
	path = defaultMediaOutputPath(path, metadata)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func expandUserPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}

func (s *TelegramService) downloadTelegramFile(ctx context.Context, cfg TelegramConfig, filePath, destination string) error {
	callCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/file/bot%s/%s", cfg.APIBase, cfg.BotToken, strings.TrimLeft(filePath, "/"))
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download telegram file failed: %s", resp.Status)
	}

	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		return err
	}
	return nil
}
