package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProviderRegistersTeamRepositoryResource(t *testing.T) {
	t.Parallel()

	found := false
	for _, factory := range (&forgejoProvider{}).Resources(context.Background()) {
		candidate := factory()
		response := resource.MetadataResponse{}
		candidate.Metadata(
			context.Background(),
			resource.MetadataRequest{ProviderTypeName: "forgejo"},
			&response,
		)
		if response.TypeName == "forgejo_team_repository" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("provider does not register forgejo_team_repository")
	}
}
