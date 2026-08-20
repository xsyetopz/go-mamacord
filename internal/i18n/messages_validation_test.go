package i18n

import "testing"

func TestValidateMessagesJSONRejectsAmbiguousOrInvalidAuthority(t *testing.T) {
	t.Parallel()
	cases := map[string]string{"duplicate_key": `[{"id":"x","id":"y","translation":"ok"}]`, "duplicate_id": `[{"id":"x","translation":"one"},{"id":"x","translation":"two"}]`, "unknown": `[{"id":"x","translation":"ok","extra":true}]`, "null": `[{"id":"x","translation":null}]`, "trailing": `[] {}`}
	for name, payload := range cases {
		name, payload := name, payload
		t.Run(name, func(t *testing.T) {
			if err := validateMessagesJSON([]byte(payload)); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	if err := validateMessagesJSON([]byte(`[{"id":"count","description":"Count","translation":{"one":"one","other":"many"}}]`)); err != nil {
		t.Fatal(err)
	}
}
