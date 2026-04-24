package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gatecli/gatecli"
	flags "github.com/jessevdk/go-flags"
)

type authCommand struct {
	service *TelegramService
	Token   string `positional-arg-name:"TOKEN" required:"true" description:"Telegram bot token"`
}

func (c *authCommand) Execute(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	path, err := c.service.configPath()
	if err != nil {
		return err
	}
	if err := writeTelegramConfigValue(path, "botToken", strings.TrimSpace(c.Token)); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, path)
	return err
}

type keyboardCommand struct {
	service *TelegramService
}

func (c *keyboardCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errors.New("expected keyboard subcommand")
	}
	switch args[0] {
	case "reply":
		cmd := &keyboardReplyCommand{service: c.service}
		parser := flags.NewParser(cmd, flags.HelpFlag|flags.PassDoubleDash)
		parser.Name = "keyboard reply"
		parser.Usage = "[command-options]"
		_, err := parser.ParseArgs(args[1:])
		if err != nil {
			if flags.WroteHelp(err) {
				return nil
			}
			return err
		}
		return cmd.run()
	case "inline":
		cmd := &keyboardInlineCommand{service: c.service}
		parser := flags.NewParser(cmd, flags.HelpFlag|flags.PassDoubleDash)
		parser.Name = "keyboard inline"
		parser.Usage = "[command-options]"
		_, err := parser.ParseArgs(args[1:])
		if err != nil {
			if flags.WroteHelp(err) {
				return nil
			}
			return err
		}
		return cmd.run()
	case "remove":
		cmd := &keyboardRemoveCommand{service: c.service}
		parser := flags.NewParser(cmd, flags.HelpFlag|flags.PassDoubleDash)
		parser.Name = "keyboard remove"
		parser.Usage = "[command-options]"
		_, err := parser.ParseArgs(args[1:])
		if err != nil {
			if flags.WroteHelp(err) {
				return nil
			}
			return err
		}
		return cmd.run()
	default:
		return fmt.Errorf("unknown keyboard subcommand %q", args[0])
	}
}

type keyboardReplyCommand struct {
	service     *TelegramService
	Target      string   `short:"t" long:"target" description:"Target chat ID"`
	ToAlias     string   `long:"to" description:"Target chat ID"`
	SuperUser   bool     `short:"u" long:"su" description:"Use configured super user"`
	Message     string   `short:"m" long:"message" required:"true" description:"Message text sent with the keyboard"`
	Rows        []string `long:"row" description:"Reply keyboard row, buttons separated by |"`
	Resize      bool     `long:"resize" description:"Request Telegram to resize the keyboard"`
	OneTime     bool     `long:"one-time" description:"Hide the keyboard after one use"`
	Persistent  bool     `long:"persistent" description:"Keep the keyboard persistent"`
	Placeholder string   `long:"placeholder" description:"Input field placeholder text"`
	Selective   bool     `long:"selective" description:"Show the keyboard only for selected users"`
}

func (c *keyboardReplyCommand) Execute(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	return c.run()
}

func (c *keyboardReplyCommand) run() error {
	target, err := resolveCommandTarget(c.service, c.Target, c.ToAlias, c.SuperUser)
	if err != nil {
		return err
	}
	markup, err := buildReplyKeyboardMarkup(c.Rows, c.Resize, c.OneTime, c.Persistent, c.Placeholder, c.Selective)
	if err != nil {
		return err
	}
	sent, err := c.service.SendTextWithReplyMarkup(context.Background(), target, c.Message, markup)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%d\n", sent.MessageID)
	return err
}

type keyboardInlineCommand struct {
	service   *TelegramService
	Target    string   `short:"t" long:"target" description:"Target chat ID"`
	ToAlias   string   `long:"to" description:"Target chat ID"`
	SuperUser bool     `short:"u" long:"su" description:"Use configured super user"`
	Message   string   `short:"m" long:"message" required:"true" description:"Message text sent with the inline keyboard"`
	Rows      []string `long:"row" description:"Inline keyboard row, buttons separated by | and written as Text=URL or Text=callback_data"`
}

func (c *keyboardInlineCommand) Execute(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	return c.run()
}

func (c *keyboardInlineCommand) run() error {
	target, err := resolveCommandTarget(c.service, c.Target, c.ToAlias, c.SuperUser)
	if err != nil {
		return err
	}
	markup, err := buildInlineKeyboardMarkup(c.Rows)
	if err != nil {
		return err
	}
	sent, err := c.service.SendTextWithReplyMarkup(context.Background(), target, c.Message, markup)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%d\n", sent.MessageID)
	return err
}

type keyboardRemoveCommand struct {
	service   *TelegramService
	Target    string `short:"t" long:"target" description:"Target chat ID"`
	ToAlias   string `long:"to" description:"Target chat ID"`
	SuperUser bool   `short:"u" long:"su" description:"Use configured super user"`
	Message   string `short:"m" long:"message" description:"Message text sent while removing the keyboard"`
	Selective bool   `long:"selective" description:"Remove the keyboard only for selected users"`
}

func (c *keyboardRemoveCommand) Execute(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	return c.run()
}

func (c *keyboardRemoveCommand) run() error {
	target, err := resolveCommandTarget(c.service, c.Target, c.ToAlias, c.SuperUser)
	if err != nil {
		return err
	}
	message := strings.TrimSpace(c.Message)
	if message == "" {
		message = "Keyboard removed."
	}
	sent, err := c.service.SendTextWithReplyMarkup(context.Background(), target, message, telegramReplyKeyboardRemove{RemoveKeyboard: true, Selective: c.Selective})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "%d\n", sent.MessageID)
	return err
}

type historyCommand struct {
	service *TelegramService
	Target  string `short:"t" long:"target" description:"Filter by chat ID"`
	ToAlias string `long:"to" description:"Filter by chat ID"`
	JSON    bool   `long:"json" description:"Output messages as JSON"`
	Limit   int    `short:"n" long:"limit" description:"Maximum number of messages to show"`
}

func (c *historyCommand) Execute(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	if c.service.app == nil {
		return errors.New("app is not initialized")
	}
	user := strings.TrimSpace(c.Target)
	if user == "" {
		user = strings.TrimSpace(c.ToAlias)
	}
	limit := c.Limit
	if limit <= 0 {
		limit = 100
	}
	messages, err := c.service.app.ListMessages(context.Background(), user, limit)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if err := writeStoredMessage(os.Stdout, message, c.JSON); err != nil {
			return err
		}
	}
	return nil
}

type meCommand struct {
	service *TelegramService
	JSON    bool `long:"json" description:"Output raw JSON"`
}

func (c *meCommand) Execute(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	user, err := c.service.GetMe(context.Background())
	if err != nil {
		return err
	}
	if c.JSON {
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(os.Stdout, "%s\n", data)
		return err
	}
	username := strings.TrimSpace(user.Username)
	if username != "" {
		username = "@" + strings.TrimPrefix(username, "@")
	}
	name := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	_, err = fmt.Fprintf(os.Stdout, "id=%d username=%s name=%s\n", user.ID, username, name)
	return err
}

type chatCommand struct {
	service *TelegramService
}

func (c *chatCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errors.New("expected chat subcommand")
	}
	if args[0] != "info" {
		return fmt.Errorf("unknown chat subcommand %q", args[0])
	}
	cmd := &chatInfoCommand{service: c.service}
	parser := flags.NewParser(cmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.Name = "chat info"
	parser.Usage = "[command-options] <id>"
	_, err := parser.ParseArgs(args[1:])
	if err != nil {
		if flags.WroteHelp(err) {
			return nil
		}
		return err
	}
	return cmd.run()
}

type chatInfoCommand struct {
	service *TelegramService
	JSON    bool   `long:"json" description:"Output raw JSON"`
	Photo   bool   `long:"photo" description:"Include profile photo information"`
	ID      string `positional-arg-name:"ID" required:"true" description:"Telegram chat ID"`
}

func (c *chatInfoCommand) Execute(args []string) error {
	return c.runWithArgs(args)
}

func (c *chatInfoCommand) runWithArgs(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}
	return c.run()
}

func (c *chatInfoCommand) run() error {
	info, err := c.service.GetChat(context.Background(), c.ID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"id":        info.ID,
		"type":      info.Type,
		"title":     info.Title,
		"username":  info.Username,
		"firstName": info.FirstName,
		"lastName":  info.LastName,
	}
	if c.Photo {
		photos, err := c.service.GetUserProfilePhotos(context.Background(), c.ID)
		if err != nil {
			return err
		}
		payload["profilePhotos"] = photos
	}
	if c.JSON {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(os.Stdout, "%s\n", data)
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "id=%d type=%s title=%s username=%s first_name=%s last_name=%s\n", info.ID, info.Type, info.Title, info.Username, info.FirstName, info.LastName)
	return err
}

type mediaCommand struct {
	service *TelegramService
}

func (c *mediaCommand) Execute(args []string) error {
	if len(args) == 0 {
		return errors.New("expected media subcommand")
	}
	if args[0] != "get" {
		return fmt.Errorf("unknown media subcommand %q", args[0])
	}
	cmd := &mediaGetCommand{}
	parser := flags.NewParser(cmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.Name = "media get"
	parser.Usage = "[command-options] <id>"
	_, err := parser.ParseArgs(args[1:])
	if err != nil {
		if flags.WroteHelp(err) {
			return nil
		}
		return err
	}
	path, err := c.service.DownloadMedia(context.Background(), cmd.ID, cmd.Output)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, path)
	return err
}

type mediaGetCommand struct {
	Output string `short:"o" long:"output" description:"Write the downloaded file to this path"`
	ID     string `positional-arg-name:"ID" required:"true" description:"Stored media UUID"`
}

func defaultMediaOutputPath(basePath string, metadata telegramMediaMetadata) string {
	path := basePath
	ext := outputExtension(metadata)
	if ext == "" {
		return path
	}
	if strings.EqualFold(filepath.Ext(path), ext) {
		return path
	}
	return path + ext
}

func outputExtension(metadata telegramMediaMetadata) string {
	if ext := strings.TrimSpace(filepath.Ext(metadata.FileName)); ext != "" {
		return strings.ToLower(ext)
	}
	format := strings.TrimSpace(metadata.Format)
	if format == "" {
		return ""
	}
	if strings.HasPrefix(format, ".") {
		return strings.ToLower(format)
	}
	return "." + strings.ToLower(format)
}

func buildReplyKeyboardMarkup(rows []string, resize bool, oneTime bool, persistent bool, placeholder string, selective bool) (telegramReplyKeyboardMarkup, error) {
	keyboard, err := parseReplyKeyboardRows(rows)
	if err != nil {
		return telegramReplyKeyboardMarkup{}, err
	}
	return telegramReplyKeyboardMarkup{
		Keyboard:              keyboard,
		IsPersistent:          persistent,
		ResizeKeyboard:        resize,
		OneTimeKeyboard:       oneTime,
		InputFieldPlaceholder: strings.TrimSpace(placeholder),
		Selective:             selective,
	}, nil
}

func parseReplyKeyboardRows(rows []string) ([][]telegramReplyKeyboardButton, error) {
	parsed := make([][]telegramReplyKeyboardButton, 0, len(rows))
	for _, rawRow := range rows {
		parts, err := splitKeyboardRow(rawRow)
		if err != nil {
			return nil, err
		}
		row := make([]telegramReplyKeyboardButton, 0, len(parts))
		for _, part := range parts {
			row = append(row, telegramReplyKeyboardButton{Text: part})
		}
		parsed = append(parsed, row)
	}
	if len(parsed) == 0 {
		return nil, errors.New("at least one --row is required")
	}
	return parsed, nil
}

func buildInlineKeyboardMarkup(rows []string) (telegramInlineKeyboardMarkup, error) {
	keyboard, err := parseInlineKeyboardRows(rows)
	if err != nil {
		return telegramInlineKeyboardMarkup{}, err
	}
	return telegramInlineKeyboardMarkup{InlineKeyboard: keyboard}, nil
}

func parseInlineKeyboardRows(rows []string) ([][]telegramInlineKeyboardButton, error) {
	parsed := make([][]telegramInlineKeyboardButton, 0, len(rows))
	for _, rawRow := range rows {
		parts, err := splitKeyboardRow(rawRow)
		if err != nil {
			return nil, err
		}
		row := make([]telegramInlineKeyboardButton, 0, len(parts))
		for _, part := range parts {
			button, err := parseInlineKeyboardButton(part)
			if err != nil {
				return nil, err
			}
			row = append(row, button)
		}
		parsed = append(parsed, row)
	}
	if len(parsed) == 0 {
		return nil, errors.New("at least one --row is required")
	}
	return parsed, nil
}

func splitKeyboardRow(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("keyboard row cannot be empty")
	}
	chunks := strings.Split(trimmed, "|")
	parts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		part := strings.TrimSpace(chunk)
		if part == "" {
			return nil, fmt.Errorf("keyboard row %q contains an empty button", raw)
		}
		parts = append(parts, part)
	}
	return parts, nil
}

func parseInlineKeyboardButton(raw string) (telegramInlineKeyboardButton, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), "=", 2)
	if len(parts) != 2 {
		return telegramInlineKeyboardButton{}, fmt.Errorf("inline button %q must be Text=URL or Text=callback_data", raw)
	}
	text := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	if text == "" || action == "" {
		return telegramInlineKeyboardButton{}, fmt.Errorf("inline button %q must include both text and action", raw)
	}
	button := telegramInlineKeyboardButton{Text: text}
	switch {
	case strings.HasPrefix(action, "url:"):
		button.URL = strings.TrimSpace(action[len("url:"):])
	case strings.HasPrefix(action, "data:"):
		button.CallbackData = strings.TrimSpace(action[len("data:"):])
	case strings.Contains(action, "://"):
		button.URL = action
	default:
		button.CallbackData = action
	}
	if button.URL == "" && button.CallbackData == "" {
		return telegramInlineKeyboardButton{}, fmt.Errorf("inline button %q must include a URL or callback data", raw)
	}
	return button, nil
}

func resolveCommandTarget(service *TelegramService, target string, alias string, useSuperUser bool) (string, error) {
	user := strings.TrimSpace(target)
	if user == "" {
		user = strings.TrimSpace(alias)
	}
	if user == "" && useSuperUser {
		if service == nil || service.app == nil {
			return "", errors.New("app is not initialized")
		}
		user = strings.TrimSpace(service.app.Config().SuperUser)
	}
	if user == "" {
		return "", errors.New("target chat is required")
	}
	return user, nil
}

func writeStoredMessage(w *os.File, message gatecli.StoredMessage, jsonMode bool) error {
	if jsonMode {
		payload := map[string]any{
			"time":    message.DateTime.Format(time.RFC3339Nano),
			"id":      message.User,
			"message": message.Items,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	}
	_, err := fmt.Fprintf(w, "[%s] id=%s message=%s\n", message.DateTime.Format(time.RFC3339Nano), message.User, gatecli.RenderMessage(message.Items))
	return err
}
