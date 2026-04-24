package main

import "encoding/json"

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

type telegramUpdate struct {
	UpdateID          int64            `json:"update_id"`
	Message           *telegramMessage `json:"message,omitempty"`
	EditedMessage     *telegramMessage `json:"edited_message,omitempty"`
	ChannelPost       *telegramMessage `json:"channel_post,omitempty"`
	EditedChannelPost *telegramMessage `json:"edited_channel_post,omitempty"`
	BusinessMessage   *telegramMessage `json:"business_message,omitempty"`
	EditedBusinessMsg *telegramMessage `json:"edited_business_message,omitempty"`
}

type telegramMessage struct {
	MessageID       int64                   `json:"message_id"`
	Date            int64                   `json:"date"`
	Chat            telegramChat            `json:"chat"`
	From            *telegramUser           `json:"from,omitempty"`
	Text            string                  `json:"text,omitempty"`
	Entities        []telegramMessageEntity `json:"entities,omitempty"`
	Caption         string                  `json:"caption,omitempty"`
	CaptionEntities []telegramMessageEntity `json:"caption_entities,omitempty"`
	Photo           []telegramPhotoSize     `json:"photo,omitempty"`
	Audio           *telegramAudio          `json:"audio,omitempty"`
	Document        *telegramDocument       `json:"document,omitempty"`
	Video           *telegramVideo          `json:"video,omitempty"`
	Voice           *telegramVoice          `json:"voice,omitempty"`
	Sticker         *telegramSticker        `json:"sticker,omitempty"`
}

type telegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type,omitempty"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type telegramMessageEntity struct {
	Type          string        `json:"type"`
	Offset        int           `json:"offset"`
	Length        int           `json:"length"`
	URL           string        `json:"url,omitempty"`
	User          *telegramUser `json:"user,omitempty"`
	CustomEmojiID string        `json:"custom_emoji_id,omitempty"`
}

type telegramPhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramAudio struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	Duration     int    `json:"duration,omitempty"`
	Performer    string `json:"performer,omitempty"`
	Title        string `json:"title,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramDocument struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramVideo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	Duration     int    `json:"duration,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramVoice struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	Duration     int    `json:"duration,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramSticker struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	Emoji        string `json:"emoji,omitempty"`
	SetName      string `json:"set_name,omitempty"`
	IsAnimated   bool   `json:"is_animated,omitempty"`
	IsVideo      bool   `json:"is_video,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramFile struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramUserProfilePhotos struct {
	TotalCount int                   `json:"total_count"`
	Photos     [][]telegramPhotoSize `json:"photos,omitempty"`
}

type telegramReplyKeyboardRemove struct {
	RemoveKeyboard bool `json:"remove_keyboard"`
	Selective      bool `json:"selective,omitempty"`
}

type telegramSentMessage struct {
	MessageID int64 `json:"message_id"`
}

type telegramReplyKeyboardButton struct {
	Text string `json:"text"`
}

type telegramReplyKeyboardMarkup struct {
	Keyboard              [][]telegramReplyKeyboardButton `json:"keyboard"`
	IsPersistent          bool                            `json:"is_persistent,omitempty"`
	ResizeKeyboard        bool                            `json:"resize_keyboard,omitempty"`
	OneTimeKeyboard       bool                            `json:"one_time_keyboard,omitempty"`
	InputFieldPlaceholder string                          `json:"input_field_placeholder,omitempty"`
	Selective             bool                            `json:"selective,omitempty"`
}

type telegramInlineKeyboardButton struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

type telegramInlineKeyboardMarkup struct {
	InlineKeyboard [][]telegramInlineKeyboardButton `json:"inline_keyboard"`
}

type telegramMediaMetadata struct {
	Type         string `json:"type"`
	FileID       string `json:"fileId"`
	FileUniqueID string `json:"fileUniqueId,omitempty"`
	FileName     string `json:"fileName,omitempty"`
	MimeType     string `json:"mimeType,omitempty"`
	FileSize     int64  `json:"fileSize,omitempty"`
	Format       string `json:"format,omitempty"`
	ChatID       int64  `json:"chatId,omitempty"`
	MessageID    int64  `json:"messageId,omitempty"`
	Date         int64  `json:"date,omitempty"`
	Emoji        string `json:"emoji,omitempty"`
}
