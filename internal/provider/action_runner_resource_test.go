package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
