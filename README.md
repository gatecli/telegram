# telegram

A Telegram CLI built on top of [`gatecli`](https://github.com/gatecli/gatecli).

It turns a Telegram Bot API backend into a usable local CLI app with:

- `send`, `watch`, `daemon` from `gatecli`
- local SQLite storage for messages and media metadata
- Telegram-specific commands such as `auth`, `me`, `chat info`, `history`, `media get`, and `keyboard`

## Features

- Uses the Telegram Bot API via long polling
- Stores message history locally through `gatecli`
- Stores Telegram media metadata locally and downloads files on demand
- Supports Telegram text and media sending
- Supports reply keyboard, inline keyboard, and keyboard removal commands
- Writes bot token into the shared `gatecli` config location

## Requirements

- Go 1.25+
- A Telegram bot token from BotFather
- A local checkout of `gatecli` during development, because this project currently uses:

```go
replace github.com/gatecli/gatecli => ../gatecli
```

See `go.mod:1-16`.

## Install

From the `telegram` project directory:

```bash
go mod tidy
go build ./...
```

Or run directly:

```bash
go run . --help
```

Entry point: `main.go:10-21`.

## Configuration

This project uses the same default config path as the `gatecli` service named `telegram`:

- config: `~/.gatecli/telegram/config.json`
- data directory: `~/.gatecli/telegram/data`
- database: `~/.gatecli/telegram/gatecli.db`

Path resolution comes from `gatecli`, while the Telegram-specific fields are loaded in `config.go:13-137`.

### Recommended first step

Write the bot token with:

```bash
telegram auth <token>
```

This updates `botToken` in the config file and prints the config path.
See `commands.go:17-35` and `config.go:115-137`.

### Example config

```json
{
  "botToken": "123456:ABCDEF",
  "apiBase": "https://api.telegram.org",
  "pollTimeout": "30s",
  "requestTimeout": "60s",
  "allowedUpdates": [
    "message",
    "edited_message",
    "channel_post",
    "edited_channel_post"
  ],
  "superUser": "123456789",
  "mediaTTL": "72h"
}
```

### Telegram-specific fields

- `botToken`: Telegram bot token, required for API calls
- `apiBase`: Bot API base URL, defaults to `https://api.telegram.org`
- `pollTimeout`: long polling timeout, defaults to `30s`
- `requestTimeout`: HTTP request timeout, defaults to `60s`
- `allowedUpdates`: Telegram update types used by `watch`/`daemon`

Default values are defined in `config.go:29-36`.

## Quick start

### 1. Write the bot token

```bash
telegram auth 123456:ABCDEF
```

### 2. Send a message

```bash
telegram send -t 123456789 "hi"
telegram send -u "hi"
```

The generic `send` command is provided by `gatecli`, and Telegram sending is implemented in `service.go:61-132`.

### 3. Watch incoming messages

```bash
telegram watch
telegram watch -t 123456789
telegram watch --json
```

Watch uses Telegram `getUpdates` long polling in `service.go:134-199`.

### 4. Run daemon mode

```bash
telegram daemon
telegram daemon --super-user 123456789
telegram daemon --media-ttl 72h
```

`daemon` comes from `gatecli` and forwards incoming messages to configured executors.

## Generic commands from gatecli

The following commands are inherited from `gatecli`:

- `send`
- `watch`
- `daemon`

Their parser registration is done by `gatecli`, and this service is attached through `gatecli.Create(...)` in `main.go:10-21`.

### send

```bash
telegram send -t <chat_id> [--json] <MSG>...
telegram send -u <MSG>...
```

Options:

- `-t`, `--target`: target chat ID
- `--to`: alias of target chat ID
- `-u`, `--su`: use configured `superUser`
- `--json`: parse each message argument as JSON message items

Message sending behavior is implemented in `service.go:61-132`.

### watch

```bash
telegram watch [-t <chat_id>] [--json]
```

Options:

- `-t`, `--target`: only output messages from one chat
- `--to`: alias of target chat ID
- `--json`: output JSON instead of text

### daemon

```bash
telegram daemon [--super-user <id>] [--data-dir <path>] [--media-ttl <duration>] [--executor <json>]
```

Options:

- `--super-user`: override configured super user
- `--data-dir`: override data directory
- `--media-ttl`: override media retention duration
- `--executor`: override executor config as JSON

## Telegram-specific commands

Command registration is implemented in `service.go:38-59`.

### auth

Write the bot token into config:

```bash
telegram auth <token>
```

Reference: `commands.go:17-35`.

### me

Show the current bot profile:

```bash
telegram me
telegram me --json
```

Reference: `commands.go:229-257`, `service.go:568-578`.

### chat info

Inspect a chat:

```bash
telegram chat info <id>
telegram chat info --json <id>
telegram chat info --photo <id>
```

`--photo` additionally calls `getUserProfilePhotos`.

Reference: `commands.go:259-331`, `service.go:611-633`.

### history

Read locally stored message history:

```bash
telegram history
telegram history -t 123456789
telegram history --json
telegram history -n 20
```

Reference: `commands.go:194-227`.

### media get

Download stored Telegram media by local media UUID:

```bash
telegram media get <uuid>
telegram media get -o /tmp/file <uuid>
```

Behavior:

- if the media was already downloaded and the file still exists, the existing path is returned
- otherwise the command calls Telegram `getFile`, downloads the content, stores it locally, and marks it downloaded in SQLite
- if the output path does not have an extension, one is inferred from metadata

Reference: `commands.go:334-379`, `service.go:522-566`, `service.go:635-692`.

### keyboard

The `keyboard` command supports three explicit subcommands:

- `keyboard reply`
- `keyboard inline`
- `keyboard remove`

Dispatcher: `commands.go:37-88`.

#### keyboard reply

Send a message with a reply keyboard:

```bash
telegram keyboard reply -t 123456789 \
  -m "请选择操作" \
  --row "开始|帮助" \
  --row "设置" \
  --resize \
  --one-time
```

Options:

- `-t`, `--target`
- `--to`
- `-u`, `--su`
- `-m`, `--message`
- `--row`
- `--resize`
- `--one-time`
- `--persistent`
- `--placeholder`
- `--selective`

Reference: `commands.go:90-126`, `commands.go:395-427`.

#### keyboard inline

Send a message with an inline keyboard:

```bash
telegram keyboard inline -t 123456789 \
  -m "Please choose" \
  --row "Home=https://example.com|Refresh=data:refresh" \
  --row "Close=close"
```

Button rules:

- `Text=https://...` -> URL button
- `Text=url:https://...` -> URL button
- `Text=data:refresh` -> callback button
- `Text=refresh` -> callback button

Reference: `commands.go:128-159`, `commands.go:429-502`.

#### keyboard remove

Remove the current reply keyboard:

```bash
telegram keyboard remove -t 123456789 -m "已关闭键盘"
```

If `-m` is omitted, the default message is `Keyboard removed.`.

Reference: `commands.go:161-191`.

## Message format

This project reuses `gatecli` message items.

Text mode example:

```text
你好[at?id=123456]
[image?format=jpg&url=https%3A%2F%2Fexample.com%2Fa.jpg]
```

JSON mode example:

```json
[
  {
    "type": "text",
    "content": "你好"
  },
  {
    "type": "at",
    "id": "123456"
  }
]
```

For Telegram sending:

- `text` and `at` are rendered to Telegram HTML
- media items such as `image`, `audio`, `document`, `video`, `voice`, `sticker`, `animation` are mapped to Telegram send methods

References:

- HTML rendering: `service.go:456-488`
- type mapping: `service.go:431-450`
- media source resolution: `service.go:404-429`

## Receiving and local storage

Incoming Telegram updates are converted into `gatecli` message items and stored locally.

Supported incoming media types currently include:

- `image`
- `audio`
- `document`
- `video`
- `voice`
- `sticker`

Conversion and metadata storage:

- update selection: `service.go:245-262`
- message conversion: `service.go:264-379`
- media metadata storage: `service.go:381-402`

## Development notes

### Local dependency

This repository currently depends on a sibling checkout of `gatecli`:

- `go.mod:16`

```go
replace github.com/gatecli/gatecli => ../gatecli
```

### Tests

Run:

```bash
go test ./...
```

Current tests cover config parsing, HTML rendering, config writing, output extension inference, keyboard parsing, target resolution, and send method mapping.
See `service_test.go:13-155`.

## Project structure

- `main.go`: CLI entry point
- `config.go`: Telegram config loading and config writing helpers
- `telegram_types.go`: Telegram Bot API data types used by this CLI
- `service.go`: `gatecli.Service` implementation and Telegram API integration
- `commands.go`: Telegram-specific CLI commands
- `service_test.go`: focused unit tests

## Current scope

This project is intentionally focused on a usable Telegram CLI on top of `gatecli`.

Implemented:

- auth
- send/watch/daemon integration
- me
- chat info
- history
- media get
- keyboard reply/inline/remove
- local metadata storage and lazy media download

Not implemented yet:

- webhook mode
- richer entity reconstruction from Telegram entities into `gatecli` items
- sticker management commands
- advanced keyboard DSL beyond current row syntax
