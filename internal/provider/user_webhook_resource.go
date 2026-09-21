package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var (
	_ resource.Resource                = &userWebhookResource{}
	_ resource.ResourceWithConfigure   = &userWebhookResource{}
	_ resource.ResourceWithImportState = &userWebhookResource{}
)

type userWebhookResource struct {
	client *forgejo.Client
}

type userWebhookResourceModel struct {
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

func (m userWebhookResourceModel) values() webhookModelValues {
	return webhookModelValues(m)
}

func (m *userWebhookResourceModel) setValues(v webhookModelValues) {
	m.WebhookID, m.Active = v.WebhookID, v.Active
	m.AuthorizationHeader, m.BranchFilter = v.AuthorizationHeader, v.BranchFilter
	m.Config, m.CreatedAt, m.Events = v.Config, v.CreatedAt, v.Events
	m.Type, m.UpdatedAt = v.Type, v.UpdatedAt
}

func (r *userWebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_webhook"
}

func (r *userWebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Forgejo authenticated-user webhook resource. Use a provider configured for the user who owns the webhook.",
		Attributes:          webhookResourceAttributes(),
	}
}

func (r *userWebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userWebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create user webhook resource"))
	var data userWebhookResourceModel
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
	tflog.Info(ctx, "Create user webhook", map[string]any{"config": redactWebhookConfig(opts.Config)})
	hook, response, err := r.client.CreateMyHook(opts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create user webhook", webhookAPIError(response, err))
		return
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userWebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read user webhook resource"))
	var data userWebhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hook, response, err := r.client.GetMyHook(data.WebhookID.ValueInt64())
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read user webhook", webhookAPIError(response, err))
		return
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userWebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update user webhook resource"))
	var data userWebhookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts, diags := data.values().editOption(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.EditMyHook(data.WebhookID.ValueInt64(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update user webhook", webhookAPIError(response, err))
		return
	}
	hook, response, err := r.client.GetMyHook(data.WebhookID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read user webhook", webhookAPIError(response, err))
		return
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userWebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete user webhook resource"))
	var data userWebhookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.DeleteMyHook(data.WebhookID.ValueInt64())
	if err != nil && (response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete user webhook", webhookAPIError(response, err))
	}
}

func (r *userWebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import user webhook resource"))
	webhookID, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse import identifier", fmt.Sprintf("Failed to parse webhook ID: %s", err))
		return
	}
	hook, response, err := r.client.GetMyHook(webhookID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read user webhook", webhookAPIError(response, err))
		return
	}
	data := userWebhookResourceModel{
		BranchFilter:        types.StringValue(""),
		AuthorizationHeader: types.StringNull(),
	}
	values, diags := webhookValuesFromHook(ctx, data.values(), hook)
	resp.Diagnostics.Append(diags...)
	data.setValues(values)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func NewUserWebhookResource() resource.Resource {
	return &userWebhookResource{}
}
