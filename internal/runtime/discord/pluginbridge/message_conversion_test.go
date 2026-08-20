package pluginbridge

import (
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"testing"
)

func TestContractMessageSuppressesAmbientMentions(t *testing.T) {
	message, err := ContractMessage("test", contract.Message{Content: "<@175928847299117063> @everyone"})
	if err != nil {
		t.Fatal(err)
	}
	if message.AllowedMentions == nil {
		t.Fatal("allowed mentions policy is absent")
	}
}
