package parse_test

import (
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/parse"
)

func TestParseEmojiID(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"123", 123, true},
		{"<:x:123>", 123, true},
		{"<a:x:123>", 123, true},
		{"", 0, false},
		{"<:x:>", 0, false},
	}
	for _, tc := range cases {
		got, ok := parse.ParseEmojiID(tc.in)
		if uint64(got) != tc.want || ok != tc.ok {
			t.Fatalf("ParseEmojiID(%q) = (%d,%v), want (%d,%v)", tc.in, uint64(got), ok, tc.want, tc.ok)
		}
	}
}

func TestParseStickerID(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"123", 123, true},
		{"<123>", 123, true},
		{"https://discord.com/stickers/123", 123, true},
		{"https://www.discord.com/stickers/123", 123, true},
		{"https://example.com/stickers/123", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parse.ParseStickerID(tc.in)
		if uint64(got) != tc.want || ok != tc.ok {
			t.Fatalf("ParseStickerID(%q) = (%d,%v), want (%d,%v)", tc.in, uint64(got), ok, tc.want, tc.ok)
		}
	}
}

func TestParseHexColor(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"#ff00aa", 0xff00aa, true},
		{"ff00aa", 0xff00aa, true},
		{"#fff", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parse.ParseHexColor(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("ParseHexColor(%q) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseMessageID(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"123", 123, true},
		{"<123>", 123, true},
		{"https://discord.com/channels/1/2/123", 123, true},
		{"https://discord.com/channels/1/2/0", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, ok := parse.ParseMessageID(tc.in)
		if uint64(got) != tc.want || ok != tc.ok {
			t.Fatalf("ParseMessageID(%q) = (%d,%v), want (%d,%v)", tc.in, uint64(got), ok, tc.want, tc.ok)
		}
	}
}
