package plugin

import (
	"testing"

	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func TestParseActionRejectsActionsForInteractions(t *testing.T) {
	t.Parallel()

	_, err := ParseAction("moderation", luaplugin.EncodedValue(`{
		"actions": [
			{
				"type": "send_dm",
				"message": {
					"content": "hi"
				}
			}
		]
	}`), false, ResponseSlash)
	if err == nil {
		t.Fatal("expected actions to be rejected for interaction responses")
	}
	if got := err.Error(); got != "actions are automation-only" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestParseActionAllowsDeferredSlashUpdate(t *testing.T) {
	t.Parallel()

	action, err := ParseAction("info", luaplugin.EncodedValue(`{
		"type": "update",
		"__deferred": true,
		"embeds": [
			{
				"title": "Lookup",
				"thumbnail_url": "https://example.com/thumb.png",
				"author": {
					"name": "MamaCord",
					"icon_url": "https://example.com/author.png"
				},
				"footer": {
					"text": "Footer",
					"icon_url": "https://example.com/footer.png"
				},
				"fields": [
					{
						"name": "Created",
						"value": "<t:1700000000:F>",
						"inline": true
					}
				]
			}
		]
	}`), true, ResponseSlash)
	if err != nil {
		t.Fatalf("ParseAction(update): %v", err)
	}
	if action.Kind != ActionUpdate {
		t.Fatalf("unexpected action kind: %#v", action)
	}
	if action.Update.Embeds == nil || len(*action.Update.Embeds) != 1 {
		t.Fatalf("expected deferred update embeds, got %#v", action.Update)
	}
	embed := (*action.Update.Embeds)[0]
	if embed.Author == nil || embed.Author.Name != "MamaCord" {
		t.Fatalf("expected author to be parsed, got %#v", embed.Author)
	}
	if embed.Footer == nil || embed.Footer.Text != "Footer" {
		t.Fatalf("expected footer to be parsed, got %#v", embed.Footer)
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != "https://example.com/thumb.png" {
		t.Fatalf("expected thumbnail to be parsed, got %#v", embed.Thumbnail)
	}
}

func TestParseActionRejectsInvalidEncodedJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseAction("broken", luaplugin.EncodedValue(`{"type":`), false, ResponseSlash)
	if err == nil {
		t.Fatal("expected invalid encoded json to fail")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected a concrete error")
	}
}

func TestParseAutomationMessageFromEncodedJSON(t *testing.T) {
	t.Parallel()

	msg, err := ParseAutomationMessage("info", luaplugin.EncodedValue(`{
		"content": "hello",
		"embeds": [
			{
				"title": "Hi"
			}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseAutomationMessage: %v", err)
	}
	if msg.Content != "hello" {
		t.Fatalf("unexpected content: %#v", msg)
	}
	if len(msg.Embeds) != 1 || msg.Embeds[0].Title != "Hi" {
		t.Fatalf("unexpected embeds: %#v", msg.Embeds)
	}
}
