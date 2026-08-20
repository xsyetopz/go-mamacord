package executor

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/disgoorg/snowflake/v2"
)

const (
	minEmojiMentionParts = 2
	minStickerLinkParts  = 2
	minMessageLinkParts  = 4
	hexColorDigits       = 6
)

func parseEmojiID(raw string) (snowflake.ID, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	// Raw snowflake.
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id != 0 {
		return snowflake.ID(id), true
	}

	// Mention format: <:name:id> or <a:name:id>
	if strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">") {
		inner := strings.Trim(raw, "<>")
		if !strings.HasPrefix(inner, ":") && !strings.HasPrefix(inner, "a:") {
			return 0, false
		}
		inner = strings.TrimPrefix(inner, "a:")
		inner = strings.TrimPrefix(inner, ":")
		parts := strings.Split(inner, ":")
		if len(parts) >= minEmojiMentionParts {
			last := parts[len(parts)-1]
			if id, err := strconv.ParseUint(last, 10, 64); err == nil && id != 0 {
				return snowflake.ID(id), true
			}
		}
	}

	return 0, false
}

func parseStickerID(raw string) (snowflake.ID, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	raw = strings.Trim(raw, "<>")

	// Raw snowflake.
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id != 0 {
		return snowflake.ID(id), true
	}

	// Link: https://discord.com/stickers/<id>
	if u, err := url.Parse(raw); err == nil && strings.TrimSpace(u.Host) != "" {
		host := strings.ToLower(strings.TrimSpace(u.Hostname()))
		if host != "discord.com" && host != "www.discord.com" {
			return 0, false
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= minStickerLinkParts && parts[len(parts)-2] == "stickers" {
			last := parts[len(parts)-1]
			if id, parseErr := strconv.ParseUint(last, 10, 64); parseErr == nil && id != 0 {
				return snowflake.ID(id), true
			}
		}
	}

	return 0, false
}

func parseHexColor(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "#")
	if len(raw) != hexColorDigits {
		return 0, false
	}

	v, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return 0, false
	}
	return int(v), true
}

func parseMessageID(raw string) (snowflake.ID, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	// Allow raw snowflake.
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id != 0 {
		return snowflake.ID(id), true
	}

	// Allow full message links: https://discord.com/channels/<guild>/<channel>/<message>
	if u, err := url.Parse(raw); err == nil && strings.TrimSpace(u.Host) != "" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= minMessageLinkParts && parts[len(parts)-1] != "" {
			last := parts[len(parts)-1]
			if id, parseErr := strconv.ParseUint(last, 10, 64); parseErr == nil && id != 0 {
				return snowflake.ID(id), true
			}
		}
	}

	// Allow <.../.../...> wrapping some clients add.
	raw = strings.Trim(raw, "<>")
	if id, err := strconv.ParseUint(raw, 10, 64); err == nil && id != 0 {
		return snowflake.ID(id), true
	}

	return 0, false
}
