package provider

import (
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/stretchr/testify/require"
)

func TestRepositoryModelRefreshesEditableSettings(t *testing.T) {
	repository := &forgejo.Repository{
		AllowManualMerge:              true,
		AutodetectManualMerge:         true,
		DefaultDeleteBranchAfterMerge: true,
		AllowFastForwardOnly:          true,
		AllowRebaseUpdate:             false,
		DefaultAllowMaintainerEdit:    true,
		DefaultUpdateStyle:            "rebase",
		EnablePrune:                   true,
		GloballyEditableWiki:          true,
		WikiBranch:                    "wiki",
	}

	var model repositoryResourceModel
	model.from(repository)

	require.True(t, model.AllowManualMerge.ValueBool())
	require.True(t, model.AutodetectManualMerge.ValueBool())
	require.True(t, model.DefaultDeleteBranchAfterMerge.ValueBool())
	require.True(t, model.AllowFastForwardOnly.ValueBool())
	require.False(t, model.AllowRebaseUpdate.ValueBool())
	require.True(t, model.DefaultAllowMaintainerEdit.ValueBool())
	require.Equal(t, "rebase", model.DefaultUpdateStyle.ValueString())
	require.True(t, model.EnablePrune.ValueBool())
	require.True(t, model.GloballyEditableWiki.ValueBool())
	require.Equal(t, "wiki", model.WikiBranch.ValueString())
}
