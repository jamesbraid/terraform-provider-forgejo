package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestBranchProtectionRuleName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		model branchProtectionResourceModel
		want  string
	}{
		{
			name: "canonical rule name",
			model: branchProtectionResourceModel{
				RuleName:   types.StringValue("release/*"),
				BranchName: types.StringValue("legacy-main"),
			},
			want: "release/*",
		},
		{
			name: "legacy branch name",
			model: branchProtectionResourceModel{
				RuleName:   types.StringNull(),
				BranchName: types.StringValue("main"),
			},
			want: "main",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, test.model.ruleName())
		})
	}
}

func branchProtectionTestResource(t *testing.T, handler http.HandlerFunc) (*branchProtectionResource, tfsdk.State) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := forgejo.NewClient(server.URL, forgejo.SetHTTPClient(server.Client()), forgejo.SetForgejoVersion("16.0.3"))
	require.NoError(t, err)
	r := &branchProtectionResource{client: client}
	var schema resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &schema)
	require.False(t, schema.Diagnostics.HasError(), "%v", schema.Diagnostics)
	return r, tfsdk.State{Schema: schema.Schema}
}

func branchProtectionTestModel() branchProtectionResourceModel {
	return branchProtectionResourceModel{
		RepositoryID:                  types.Int64Null(),
		BranchName:                    types.StringNull(),
		RuleName:                      types.StringNull(),
		EnablePush:                    types.BoolNull(),
		EnablePushWhitelist:           types.BoolNull(),
		PushWhitelistUsernames:        types.SetNull(types.StringType),
		PushWhitelistTeams:            types.SetNull(types.StringType),
		PushWhitelistDeployKeys:       types.BoolNull(),
		EnableStatusCheck:             types.BoolNull(),
		StatusCheckContexts:           types.ListNull(types.StringType),
		RequireSignedCommits:          types.BoolNull(),
		ProtectedFilePatterns:         types.StringNull(),
		UnprotectedFilePatterns:       types.StringNull(),
		EnableMergeWhitelist:          types.BoolNull(),
		MergeWhitelistUsernames:       types.SetNull(types.StringType),
		MergeWhitelistTeams:           types.SetNull(types.StringType),
		EnableApprovalsWhitelist:      types.BoolNull(),
		ApprovalsWhitelistUsernames:   types.SetNull(types.StringType),
		ApprovalsWhitelistTeams:       types.SetNull(types.StringType),
		RequiredApprovals:             types.Int64Null(),
		BlockOnRejectedReviews:        types.BoolNull(),
		BlockOnOfficialReviewRequests: types.BoolNull(),
		BlockOnOutdatedBranch:         types.BoolNull(),
		DismissStaleApprovals:         types.BoolNull(),
	}
}

func TestBranchProtectionLegacyStateRefreshesByRuleName(t *testing.T) {
	t.Parallel()

	var protectionPath string
	r, state := branchProtectionTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/repositories/7":
			fmt.Fprint(w, `{"id":7,"name":"example","owner":{"login":"infra"}}`)
		default:
			protectionPath = req.URL.EscapedPath()
			fmt.Fprint(w, `{"branch_name":"main","rule_name":"main"}`)
		}
	})
	data := branchProtectionTestModel()
	data.RepositoryID = types.Int64Value(7)
	data.BranchName = types.StringValue("main")
	require.False(t, state.Set(t.Context(), &data).HasError())
	response := resource.ReadResponse{State: state}
	r.Read(t.Context(), resource.ReadRequest{State: state}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	require.Equal(t, "/api/v1/repos/infra/example/branch_protections/main", protectionPath)
	require.False(t, response.State.Get(t.Context(), &data).HasError())
	require.Equal(t, "main", data.RuleName.ValueString())
	require.Equal(t, "main", data.BranchName.ValueString())
}

func TestBranchProtectionImportSupportsGlobRuleName(t *testing.T) {
	t.Parallel()

	var protectionPath string
	r, state := branchProtectionTestResource(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/v1/repos/infra/example":
			fmt.Fprint(w, `{"id":7,"name":"example","owner":{"login":"infra"}}`)
		default:
			protectionPath = req.URL.EscapedPath()
			fmt.Fprint(w, `{"branch_name":"","rule_name":"release/*"}`)
		}
	})
	response := resource.ImportStateResponse{State: state}
	r.ImportState(t.Context(), resource.ImportStateRequest{ID: "infra/example/release/*"}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	require.Equal(t, "/api/v1/repos/infra/example/branch_protections/release%2F%2A", protectionPath)
	var data branchProtectionResourceModel
	require.False(t, response.State.Get(t.Context(), &data).HasError())
	require.Equal(t, "release/*", data.RuleName.ValueString())
	require.Equal(t, "release/*", data.BranchName.ValueString())
}

func TestBranchProtectionCreateUsesRuleName(t *testing.T) {
	t.Parallel()

	resource := &branchProtectionResource{}
	data := branchProtectionResourceModel{
		RuleName:                    types.StringValue("release/*"),
		BranchName:                  types.StringNull(),
		PushWhitelistUsernames:      types.SetNull(types.StringType),
		PushWhitelistTeams:          types.SetNull(types.StringType),
		StatusCheckContexts:         types.ListNull(types.StringType),
		MergeWhitelistUsernames:     types.SetNull(types.StringType),
		MergeWhitelistTeams:         types.SetNull(types.StringType),
		ApprovalsWhitelistUsernames: types.SetNull(types.StringType),
		ApprovalsWhitelistTeams:     types.SetNull(types.StringType),
	}

	option := resource.toCreateOption(context.Background(), &data)
	require.Equal(t, "release/*", option.RuleName)
	require.Empty(t, option.BranchName)
}

func TestBranchProtectionRefreshCanonicalizesRuleName(t *testing.T) {
	t.Parallel()

	resource := &branchProtectionResource{}
	data := branchProtectionResourceModel{
		BranchName: types.StringValue("main"),
		RuleName:   types.StringNull(),
	}
	diagnostics := resource.from(&forgejo.BranchProtection{
		BranchName: "main",
		RuleName:   "release/*",
	}, &data)

	require.False(t, diagnostics.HasError())
	require.Equal(t, "release/*", data.RuleName.ValueString())
	require.Equal(t, "release/*", data.BranchName.ValueString())
}

func TestParseBranchProtectionImportID(t *testing.T) {
	t.Parallel()

	owner, repository, rule, err := parseBranchProtectionImportID("infra/example/release/*")
	require.NoError(t, err)
	require.Equal(t, "infra", owner)
	require.Equal(t, "example", repository)
	require.Equal(t, "release/*", rule)

	for _, id := range []string{"infra/example", "/example/main", "infra//main", "infra/example/"} {
		_, _, _, err := parseBranchProtectionImportID(id)
		require.Error(t, err, id)
	}
}
