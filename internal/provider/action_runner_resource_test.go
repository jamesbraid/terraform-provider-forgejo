package provider

import (
	"context"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestActionRunnerStateRefreshPreservesToken(t *testing.T) {
	t.Parallel()

	model := actionRunnerResourceModel{Token: types.StringValue("one-time-token")}
	diagnostics := model.from(context.Background(), &forgejo.ActionRunner{
		ID:     7,
		UUID:   "runner-uuid",
		Name:   "runner",
		Labels: []string{"docker:docker://node:22"},
		Status: "offline",
	})

	require.False(t, diagnostics.HasError())
	require.Equal(t, "one-time-token", model.Token.ValueString())
}

func TestParseActionRunnerImportID(t *testing.T) {
	t.Parallel()

	global, err := parseActionRunnerImportID("global/7")
	require.NoError(t, err)
	require.Equal(t, actionRunnerScopeGlobal, global.Scope.ValueString())
	require.True(t, global.Owner.IsNull())
	require.Equal(t, int64(7), global.ID.ValueInt64())

	organization, err := parseActionRunnerImportID("organization/infra/8")
	require.NoError(t, err)
	require.Equal(t, actionRunnerScopeOrganization, organization.Scope.ValueString())
	require.Equal(t, "infra", organization.Owner.ValueString())
	require.Equal(t, int64(8), organization.ID.ValueInt64())
}

func TestParseActionRunnerImportIDRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"global", "global/0", "global/owner/7", "organization//8", "organization/infra/nope", "repository/infra/8"} {
		_, err := parseActionRunnerImportID(id)
		require.Error(t, err, id)
	}
}
