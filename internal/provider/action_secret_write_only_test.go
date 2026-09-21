package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type actionSecretSchemaResource interface {
	Schema(context.Context, resource.SchemaRequest, *resource.SchemaResponse)
}

func TestActionSecretSchemasExposeWriteOnlyData(t *testing.T) {
	t.Parallel()

	tests := map[string]actionSecretSchemaResource{
		"organization": &organizationActionSecretResource{},
		"repository":   &repositoryActionSecretResource{},
	}

	for name, secretResource := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var response resource.SchemaResponse
			secretResource.Schema(context.Background(), resource.SchemaRequest{}, &response)

			legacy, ok := response.Schema.Attributes["data"].(schema.StringAttribute)
			if !ok {
				t.Fatal("data is not a string attribute")
			}
			if !legacy.Optional || legacy.Required || !legacy.Sensitive || legacy.WriteOnly {
				t.Fatalf("data must remain an optional, stateful sensitive attribute: %#v", legacy)
			}

			writeOnly, ok := response.Schema.Attributes["data_wo"].(schema.StringAttribute)
			if !ok {
				t.Fatal("data_wo is not a string attribute")
			}
			if !writeOnly.Optional || writeOnly.Required || !writeOnly.Sensitive || !writeOnly.WriteOnly {
				t.Fatalf("data_wo must be an optional write-only sensitive attribute: %#v", writeOnly)
			}
			if len(writeOnly.Validators) != 1 {
				t.Fatalf("data_wo must not require data_wo_version: %#v", writeOnly.Validators)
			}

			version, ok := response.Schema.Attributes["data_wo_version"].(schema.Int64Attribute)
			if !ok {
				t.Fatal("data_wo_version is not an int64 attribute")
			}
			if !version.Optional || version.Required || version.Computed || version.WriteOnly {
				t.Fatalf("data_wo_version must be optional and stateful: %#v", version)
			}
			if len(version.Validators) != 1 {
				t.Fatalf("data_wo_version must continue to require data_wo: %#v", version.Validators)
			}
		})
	}
}
