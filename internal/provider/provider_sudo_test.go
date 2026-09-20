package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
)

func TestProviderSchemaExposesSudo(t *testing.T) {
	t.Parallel()

	var response provider.SchemaResponse
	(&forgejoProvider{}).Schema(context.Background(), provider.SchemaRequest{}, &response)

	sudo, ok := response.Schema.Attributes["sudo"].(schema.StringAttribute)
	if !ok {
		t.Fatal("sudo is not a string attribute")
	}
	if !sudo.Optional || sudo.Required || sudo.Sensitive {
		t.Fatalf("sudo must be an optional, non-sensitive string attribute: %#v", sudo)
	}
}
