package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var (
	_ resource.Resource                = &teamRepositoryResource{}
	_ resource.ResourceWithConfigure   = &teamRepositoryResource{}
	_ resource.ResourceWithImportState = &teamRepositoryResource{}
)

type teamRepositoryResource struct {
	client *forgejo.Client
}

type teamRepositoryResourceModel struct {
	TeamID       types.Int64 `tfsdk:"team_id"`
	RepositoryID types.Int64 `tfsdk:"repository_id"`
}

func (r *teamRepositoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_repository"
}

func (r *teamRepositoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Forgejo team repository resource.",
		Attributes: map[string]schema.Attribute{
			"team_id": schema.Int64Attribute{
				Description: "Numeric identifier of the team. Changing this forces a new resource to be created.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"repository_id": schema.Int64Attribute{
				Description: "Numeric identifier of the repository. Changing this forces a new resource to be created.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *teamRepositoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*forgejo.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *forgejo.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *teamRepositoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create team repository resource"))
	var data teamRepositoryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(assignTeamRepository(ctx, r.client, data.TeamID.ValueInt64(), data.RepositoryID.ValueInt64())...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *teamRepositoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read team repository resource"))
	var data teamRepositoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	present, diags := teamHasRepository(ctx, r.client, data.TeamID.ValueInt64(), data.RepositoryID.ValueInt64())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if present {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *teamRepositoryResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	// Both writable attributes require replacement.
}

func (r *teamRepositoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete team repository resource"))
	var data teamRepositoryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(unassignTeamRepository(r.client, data.TeamID.ValueInt64(), data.RepositoryID.ValueInt64())...)
}

func (r *teamRepositoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	teamID, repositoryID, err := parseTeamRepositoryImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse import identifier", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &teamRepositoryResourceModel{
		TeamID:       types.Int64Value(teamID),
		RepositoryID: types.Int64Value(repositoryID),
	})...)
}

func NewTeamRepositoryResource() resource.Resource {
	return &teamRepositoryResource{}
}

func parseTeamRepositoryImportID(id string) (int64, int64, error) {
	parts := strings.Split(id, "/")
	if len(parts) == 2 {
		teamID, teamErr := strconv.ParseInt(parts[0], 10, 64)
		repositoryID, repositoryErr := strconv.ParseInt(parts[1], 10, 64)
		if teamErr == nil && repositoryErr == nil && teamID > 0 && repositoryID > 0 {
			return teamID, repositoryID, nil
		}
	}
	return 0, 0, fmt.Errorf("expected import identifier with format 'team_id/repository_id', got %q", id)
}

func assignTeamRepository(ctx context.Context, client *forgejo.Client, teamID, repositoryID int64) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	repository, repositoryDiagnostics := getRepositoryByID(ctx, client, repositoryID)
	diagnostics.Append(repositoryDiagnostics...)
	if diagnostics.HasError() {
		return diagnostics
	}

	tflog.Info(ctx, "Add repository to team", map[string]any{
		"team_id": teamID,
		"owner":   repository.Owner.UserName,
		"repo":    repository.Name,
	})
	response, err := client.AddTeamRepository(teamID, repository.Owner.UserName, repository.Name)
	if err != nil {
		diagnostics.AddError("Unable to add repository to team", apiErrorDetail(response, err))
	}
	return diagnostics
}

func teamHasRepository(_ context.Context, client *forgejo.Client, teamID, repositoryID int64) (bool, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	page := 1
	for {
		repositories, response, err := client.ListTeamRepositories(
			teamID,
			forgejo.ListTeamRepositoriesOptions{ListOptions: forgejo.ListOptions{Page: page, PageSize: 50}},
		)
		if err != nil {
			if response != nil && response.StatusCode == 404 {
				return false, diagnostics
			}
			diagnostics.AddError("Unable to read team repositories", apiErrorDetail(response, err))
			return false, diagnostics
		}
		for _, repository := range repositories {
			if repository.ID == repositoryID {
				return true, diagnostics
			}
		}
		if response == nil || response.NextPage == 0 {
			return false, diagnostics
		}
		page = response.NextPage
	}
}

func unassignTeamRepository(client *forgejo.Client, teamID, repositoryID int64) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	repository, response, err := client.GetRepoByID(repositoryID)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			return diagnostics
		}
		diagnostics.AddError("Unable to read repository", apiErrorDetail(response, err))
		return diagnostics
	}
	response, err = client.RemoveTeamRepository(teamID, repository.Owner.UserName, repository.Name)
	if err != nil && (response == nil || response.StatusCode != 404) {
		diagnostics.AddError("Unable to remove repository from team", apiErrorDetail(response, err))
	}
	return diagnostics
}

func apiErrorDetail(resp *forgejo.Response, err error) string {
	if resp == nil {
		return fmt.Sprintf("Unknown error with nil response: %s", err)
	}
	return fmt.Sprintf("Forgejo API returned status %d: %s", resp.StatusCode, err)
}
