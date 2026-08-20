package httpjson

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func TestValidateResolvedIPsRejectsNonPublicAddresses(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1", "ff02::1", "0.0.0.0"} {
		if err := validateResolvedIPs([]netip.Addr{netip.MustParseAddr(raw)}); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if err := validateResolvedIPs([]netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("2606:4700:4700::1111")}); err != nil {
		t.Fatalf("public addresses: %v", err)
	}
	if err := validateResolvedIPs([]netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")}); err == nil {
		t.Fatal("accepted mixed public/private resolution")
	}
}
func TestConvertJSONIsBoundedAndDeterministic(t *testing.T) {
	t.Parallel()
	items := 0
	value, err := convertJSON(map[string]any{"z": json.Number("1"), "a": "x"}, 0, &items)
	if err != nil {
		t.Fatal(err)
	}
	fields, ok := value.Object()
	if !ok {
		t.Fatal("not object")
	}
	if got := []string{fields[0].Key, fields[1].Key}; !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("keys=%v", got)
	}
	tooMany := make([]any, contract.MaxValueItems+1)
	items = 0
	if _, err := convertJSON(tooMany, 0, &items); err == nil {
		t.Fatal("accepted excessive JSON items")
	}
}

func TestValidateUniqueJSONKeysRejectsDuplicatesAndTrailingValues(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{`{"a":1,"a":2}`, `{"nested":{"x":1,"x":2}}`, `[1] [2]`} {
		if err := validateUniqueJSONKeys([]byte(payload)); err == nil {
			t.Errorf("accepted %s", payload)
		}
	}
	if err := validateUniqueJSONKeys([]byte(`{"a":[1,{"b":2}]}`)); err != nil {
		t.Fatal(err)
	}
}
