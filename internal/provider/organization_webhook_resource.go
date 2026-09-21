package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var (
	_ resource.Resource                = &organizationWebhookResource{}
	_ resource.ResourceWithConfigure   = &organizationWebhookResource{}
	_ resource.ResourceWithImportState = &organizationWebhookResource{}
)

type organizationWebhookResource struct {
	client *forgejo.Client
}

type organizationWebhookResourceModel struct {
	Organization        types.String `tfsdk:"organization"`
	WebhookID           types.Int64  `tfsdk:"webhook_id"`
	Active              types.Bool   `tfsdk:"active"`
	AuthorizationHeader types.String `tfsdk:"authorization_header"`
	BranchFilter        types.String `tfsdk:"branch_filter"`
	Config              types.Map    `tfsdk:"config"`
	CreatedAt           types.String `tfsdk:"created_at"`
	Events              types.Set    `tfsdk:"events"`
	Type                types.String `tfsdk:"type"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
}

func (m organizationWebhookResourceModel) values() webhookModelValues {
	return webhookModelValues{
		WebhookID: m.WebhookID, Active: m.Active,
		AuthorizationHeader: m.AuthorizationHeader, BranchFilter: m.BranchFilter,
		Config: m.Config, CreatedAt: m.CreatedAt, Events: m.Events,
		Type: m.Type, UpdatedAt: m.UpdatedAt,
	}
}

func (m *organizationWebhookResourceModel) setValues(v webhookModelValues) {
	m.WebhookID, m.Active = v.WebhookID, v.Active
	m.AuthorizationHeader, m.BranchFilter = v.AuthorizationHeader, v.BranchFilter
	m.Config, m.CreatedAt, m.Events = v.Config, v.CreatedAt, v.Events
	m.Type, m.UpdatedAt = v.Type, v.UpdatedAt
}

func (r *organizationWebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_webhook"
}

func (r *organizationWebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := webhookResourceAttributes()
	attributes["organization"] = schema.StringAttribute{
		Description: "Organization which owns the webhook. Changing this forces a new resource to be created.",
		Required:    true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Forgejo organization webhook resource.",
		Attributes:          attributes,
	}
}

func (r *organizationWebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*forgejo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *forgejo.Client, got: %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *organizationWebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create organization webhook resource"))
	var data organizationWebhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts, diags := data.values().createOption(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := opts.Validate(); err != nil {
		resp.Diagnostics.AddError("Input validation error", err.Error())
		return
	}
	tflog.Info(ctx, "Create organization webhook", map[string]any{"organization": data.Organization.ValueString(), "config": redactWebhookConfig(opts.Config)})
	hook, response, err := r.client.CreateOrgHook(data.Organization.ValueString(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create organization webhook", forgejoAPIError(response, err))
		return
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationWebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read organization webhook resource"))
	var data organizationWebhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hook, response, err := r.client.GetOrgHook(data.Organization.ValueString(), data.WebhookID.ValueInt64())
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read organization webhook", forgejoAPIError(response, err))
		return
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationWebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update organization webhook resource"))
	var data organizationWebhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts, diags := data.values().editOption(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.EditOrgHook(data.Organization.ValueString(), data.WebhookID.ValueInt64(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update organization webhook", forgejoAPIError(response, err))
		return
	}
	hook, response, err := r.client.GetOrgHook(data.Organization.ValueString(), data.WebhookID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read organization webhook", forgejoAPIError(response, err))
		return
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationWebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete organization webhook resource"))
	var data organizationWebhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.DeleteOrgHook(data.Organization.ValueString(), data.WebhookID.ValueInt64())
	if err != nil && (response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete organization webhook", forgejoAPIError(response, err))
	}
}

func (r *organizationWebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import organization webhook resource"))
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Unable to parse import identifier", fmt.Sprintf("Expected import identifier with format: 'organization/webhookID', got: '%s'", req.ID))
		return
	}
	webhookID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse import identifier", fmt.Sprintf("Failed to parse webhook ID: %s", err))
		return
	}
	hook, response, err := r.client.GetOrgHook(parts[0], webhookID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read organization webhook", forgejoAPIError(response, err))
		return
	}
	data := organizationWebhookResourceModel{
		Organization:        types.StringValue(parts[0]),
		BranchFilter:        types.StringValue(""),
		AuthorizationHeader: types.StringNull(),
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func NewOrganizationWebhookResource() resource.Resource {
	return &organizationWebhookResource{}
}
