package plugin

import "testing"

func TestParseActionRejectsActionsForInteractions(t *testing.T) {
	t.Parallel()

	_, err := ParseAction("moderation", map[string]any{
		"actions": []any{
			map[string]any{
				"type": "send_dm",
				"message": map[string]any{
					"content": "hi",
				},
			},
		},
	}, false, ResponseSlash)
	if err == nil {
		t.Fatal("expected actions to be rejected for interaction responses")
	}
	if got := err.Error(); got != "actions are automation-only" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestParseActionAllowsDeferredSlashUpdate(t *testing.T) {
	t.Parallel()

	action, err := ParseAction("info", map[string]any{
		"type":       "update",
		"__deferred": true,
		"embeds": []any{
			map[string]any{
				"title":         "Lookup",
				"thumbnail_url": "https://example.com/thumb.png",
				"author": map[string]any{
					"name":     "MamaCord",
					"icon_url": "https://example.com/author.png",
				},
				"footer": map[string]any{
					"text":     "Footer",
					"icon_url": "https://example.com/footer.png",
				},
				"fields": []any{
					map[string]any{
						"name":   "Created",
						"value":  "<t:1700000000:F>",
						"inline": true,
					},
				},
			},
		},
	}, true, ResponseSlash)
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
