package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const (
	actionRunnerScopeGlobal       = "global"
	actionRunnerScopeOrganization = "organization"
)

var (
	_ resource.Resource                   = &actionRunnerResource{}
	_ resource.ResourceWithConfigure      = &actionRunnerResource{}
	_ resource.ResourceWithImportState    = &actionRunnerResource{}
	_ resource.ResourceWithValidateConfig = &actionRunnerResource{}
)

type actionRunnerResource struct {
	client *forgejo.Client
}

type actionRunnerResourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Scope       types.String `tfsdk:"scope"`
	Owner       types.String `tfsdk:"owner"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Ephemeral   types.Bool   `tfsdk:"ephemeral"`
	UUID        types.String `tfsdk:"uuid"`
	Token       types.String `tfsdk:"token"`
	Labels      types.Set    `tfsdk:"labels"`
	Status      types.String `tfsdk:"status"`
	Version     types.String `tfsdk:"version"`
}

func (m *actionRunnerResourceModel) from(ctx context.Context, runner *forgejo.ActionRunner) (diags diag.Diagnostics) {
	if runner == nil {
		return diags
	}
	m.ID = types.Int64Value(runner.ID)
	m.Name = types.StringValue(runner.Name)
	m.Description = types.StringValue(runner.Description)
	m.Ephemeral = types.BoolValue(runner.Ephemeral)
	m.UUID = types.StringValue(runner.UUID)
	m.Status = types.StringValue(runner.Status)
	m.Version = types.StringValue(runner.Version)
	labels, d := types.SetValueFrom(ctx, types.StringType, runner.Labels)
	diags.Append(d...)
	m.Labels = labels
	return diags
}

func (r *actionRunnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action_runner"
}

func actionRunnerStringReplace() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func (r *actionRunnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a global or organization-scoped Forgejo Actions runner identity. Runner processes and their self-reported labels remain outside this resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "Numeric runner identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"scope": schema.StringAttribute{
				Description:   "Runner scope: global or organization.",
				Required:      true,
				PlanModifiers: actionRunnerStringReplace(),
				Validators: []validator.String{
					stringvalidator.OneOf(actionRunnerScopeGlobal, actionRunnerScopeOrganization),
				},
			},
			"owner": schema.StringAttribute{
				Description:   "Organization name for an organization-scoped runner; omit for a global runner.",
				Optional:      true,
				PlanModifiers: actionRunnerStringReplace(),
			},
			"name": schema.StringAttribute{
				Description:   "Runner name.",
				Required:      true,
				PlanModifiers: actionRunnerStringReplace(),
			},
			"description": schema.StringAttribute{
				Description: "Runner description.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ephemeral": schema.BoolAttribute{
				Description: "Whether Forgejo removes the runner after it completes one job.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"uuid": schema.StringAttribute{
				Description: "Runner UUID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Description: "Runner authentication token, returned only when Forgejo creates the runner.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"labels": schema.SetAttribute{
				Description: "Labels self-reported by the connected runner.",
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Runner connection status.",
				Computed:    true,
			},
			"version": schema.StringAttribute{
				Description: "Runner-reported version.",
				Computed:    true,
			},
		},
	}
}

func (r *actionRunnerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data actionRunnerResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.Scope.IsUnknown() || data.Owner.IsUnknown() {
		return
	}
	scope := data.Scope.ValueString()
	owner := data.Owner.ValueString()
	if scope == actionRunnerScopeGlobal && owner != "" {
		resp.Diagnostics.AddAttributeError(path.Root("owner"), "Unexpected runner owner", "A global runner must not declare an owner.")
	}
	if scope == actionRunnerScopeOrganization && owner == "" {
		resp.Diagnostics.AddAttributeError(path.Root("owner"), "Missing runner owner", "An organization-scoped runner must declare its organization name as owner.")
	}
}

func (r *actionRunnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureAvatarResource(req, resp)
}

func (r *actionRunnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data actionRunnerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	option := forgejo.RegisterActionRunnerOption{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Ephemeral:   data.Ephemeral.ValueBool(),
	}
	var registered *forgejo.RegisterActionRunnerResponse
	var response *forgejo.Response
	var err error
	if data.Scope.ValueString() == actionRunnerScopeGlobal {
		registered, response, err = r.client.RegisterAdminActionRunner(option)
	} else {
		registered, response, err = r.client.RegisterOrgActionRunner(data.Owner.ValueString(), option)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to register Forgejo Actions runner", forgejoAPIError(response, err))
		return
	}
	data.ID = types.Int64Value(registered.ID)
	data.UUID = types.StringValue(registered.UUID)
	data.Token = types.StringValue(registered.Token)
	// Persist the identity and one-time token before refreshing fields that are
	// only available from the runner endpoint. If that read fails, Terraform can
	// retry it without registering a second runner or losing the token.
	data.Labels = types.SetNull(types.StringType)
	data.Status = types.StringNull()
	data.Version = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	runner, response, err := r.get(data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read registered Forgejo Actions runner", forgejoAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, runner)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *actionRunnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data actionRunnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	runner, response, err := r.get(data)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forgejo Actions runner", forgejoAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, runner)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *actionRunnerResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to update Forgejo Actions runner", "Forgejo does not support editing runner identity settings; this change should have planned a replacement.")
}

func (r *actionRunnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data actionRunnerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.delete(data)
	if err != nil && (response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete Forgejo Actions runner", forgejoAPIError(response, err))
	}
}

func (r *actionRunnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	data, err := parseActionRunnerImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse import identifier", err.Error())
		return
	}
	runner, response, err := r.get(data)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forgejo Actions runner", forgejoAPIError(response, err))
		return
	}
	data.Token = types.StringNull()
	resp.Diagnostics.Append(data.from(ctx, runner)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *actionRunnerResource) get(data actionRunnerResourceModel) (*forgejo.ActionRunner, *forgejo.Response, error) {
	if data.Scope.ValueString() == actionRunnerScopeGlobal {
		return r.client.GetAdminActionRunner(data.ID.ValueInt64())
	}
	return r.client.GetOrgActionRunner(data.Owner.ValueString(), data.ID.ValueInt64())
}

func (r *actionRunnerResource) delete(data actionRunnerResourceModel) (*forgejo.Response, error) {
	if data.Scope.ValueString() == actionRunnerScopeGlobal {
		return r.client.DeleteAdminActionRunner(data.ID.ValueInt64())
	}
	return r.client.DeleteOrgActionRunner(data.Owner.ValueString(), data.ID.ValueInt64())
}

func parseActionRunnerImportID(id string) (actionRunnerResourceModel, error) {
	parts := strings.Split(id, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return actionRunnerResourceModel{}, fmt.Errorf("expected 'global/id' or 'organization/owner/id', got %q", id)
	}
	scope := parts[0]
	var owner string
	var rawID string
	switch scope {
	case actionRunnerScopeGlobal:
		if len(parts) != 2 {
			return actionRunnerResourceModel{}, fmt.Errorf("expected 'global/id', got %q", id)
		}
		rawID = parts[1]
	case actionRunnerScopeOrganization:
		if len(parts) != 3 || parts[1] == "" {
			return actionRunnerResourceModel{}, fmt.Errorf("expected 'organization/owner/id', got %q", id)
		}
		owner = parts[1]
		rawID = parts[2]
	default:
		return actionRunnerResourceModel{}, fmt.Errorf("scope must be global or organization, got %q", scope)
	}
	idNumber, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || idNumber <= 0 {
		return actionRunnerResourceModel{}, fmt.Errorf("runner ID must be a positive integer, got %q", rawID)
	}
	model := actionRunnerResourceModel{
		ID:    types.Int64Value(idNumber),
		Scope: types.StringValue(scope),
		Owner: types.StringNull(),
	}
	if owner != "" {
		model.Owner = types.StringValue(owner)
	}
	return model, nil
}

func NewActionRunnerResource() resource.Resource {
	return &actionRunnerResource{}
}
