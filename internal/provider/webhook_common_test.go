package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestNormalizedWebhookEventsPreservesDeclaredSubset(t *testing.T) {
	prior := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("issue_comment"),
		types.StringValue("pull_request"),
	})

	got, diags := normalizedWebhookEvents(context.Background(), prior, []string{
		"issue_comment",
		"pull_request",
		"pull_request_assign",
		"pull_request_sync",
	})

	require.False(t, diags.HasError())
	require.True(t, got.Equal(prior))
}

func TestNormalizedWebhookEventsReportsRemovedEvent(t *testing.T) {
	prior := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("issue_comment"),
		types.StringValue("pull_request"),
	})

	got, diags := normalizedWebhookEvents(context.Background(), prior, []string{"pull_request"})

	require.False(t, diags.HasError())
	want := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("pull_request")})
	require.True(t, got.Equal(want))
}

func TestNormalizedWebhookEventsCanonicalizesAPIExpansionOnImport(t *testing.T) {
	got, diags := normalizedWebhookEvents(context.Background(), types.SetNull(types.StringType), []string{
		"issue_comment",
		"pull_request",
		"pull_request_assign",
		"pull_request_comment",
		"pull_request_label",
		"pull_request_milestone",
		"pull_request_review_approved",
		"pull_request_review_comment",
		"pull_request_review_rejected",
		"pull_request_review_request",
		"pull_request_sync",
	})

	require.False(t, diags.HasError())
	want := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("issue_comment"),
		types.StringValue("pull_request"),
	})
	require.True(t, got.Equal(want))
}
