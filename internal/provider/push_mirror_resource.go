package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var (
	_ resource.Resource                = &pushMirrorResource{}
	_ resource.ResourceWithConfigure   = &pushMirrorResource{}
	_ resource.ResourceWithImportState = &pushMirrorResource{}
)

type pushMirrorResource struct {
	client *forgejo.Client
}

type pushMirrorResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Owner                 types.String `tfsdk:"owner"`
	Repository            types.String `tfsdk:"repository"`
	RemoteName            types.String `tfsdk:"remote_name"`
	RemoteAddress         types.String `tfsdk:"remote_address"`
	RemoteUsername        types.String `tfsdk:"remote_username"`
	RemotePasswordWO      types.String `tfsdk:"remote_password_wo"`
	RemotePasswordVersion types.Int64  `tfsdk:"remote_password_wo_version"`
	BranchFilter          types.String `tfsdk:"branch_filter"`
	Interval              types.String `tfsdk:"interval"`
	SyncOnCommit          types.Bool   `tfsdk:"sync_on_commit"`
	PublicKey             types.String `tfsdk:"public_key"`
	Created               types.String `tfsdk:"created"`
	LastUpdate            types.String `tfsdk:"last_update"`
	LastError             types.String `tfsdk:"last_error"`
}

func (m *pushMirrorResourceModel) from(mirror *forgejo.PushMirrorResponse) {
	if mirror == nil {
		return
	}
	m.ID = types.StringValue(mirror.RemoteName)
	m.RemoteName = types.StringValue(mirror.RemoteName)
	m.RemoteAddress = types.StringValue(mirror.RemoteAddress)
	m.BranchFilter = types.StringValue(mirror.BranchFilter)
	m.Interval = types.StringValue(mirror.Interval)
	m.SyncOnCommit = types.BoolValue(mirror.SyncONCommit)
	m.PublicKey = types.StringValue(mirror.PublicKey)
	m.Created = types.StringValue(mirror.Created)
	m.LastUpdate = types.StringValue(mirror.LastUpdate)
	m.LastError = types.StringValue(mirror.LastError)
}

func (r *pushMirrorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_push_mirror"
}

func pushMirrorReplace() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func (r *pushMirrorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an explicitly declared Forgejo repository push mirror. Forgejo cannot edit push mirrors, so changes replace the mirror.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"owner": schema.StringAttribute{
				Description:   "Repository owner.",
				Required:      true,
				PlanModifiers: pushMirrorReplace(),
			},
			"repository": schema.StringAttribute{
				Description:   "Repository name.",
				Required:      true,
				PlanModifiers: pushMirrorReplace(),
			},
			"remote_name": schema.StringAttribute{
				Description: "Forgejo-generated remote name used as the stable server-side identifier.",
				Computed:    true,
			},
			"remote_address": schema.StringAttribute{
				Description:   "Destination repository URL. Forgejo returns this URL without credentials.",
				Required:      true,
				PlanModifiers: pushMirrorReplace(),
			},
			"remote_username": schema.StringAttribute{
				Description:   "Username used to authenticate to the destination.",
				Required:      true,
				PlanModifiers: pushMirrorReplace(),
			},
			"remote_password_wo": schema.StringAttribute{
				Description: "Write-only password or token used to authenticate to the destination.",
				Required:    true,
				Sensitive:   true,
				WriteOnly:   true,
			},
			"remote_password_wo_version": schema.Int64Attribute{
				Description: "Version of remote_password_wo. Changing it replaces the push mirror.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"branch_filter": schema.StringAttribute{
				Description:   "Comma-separated branch filter. An empty value mirrors every branch.",
				Required:      true,
				PlanModifiers: pushMirrorReplace(),
			},
			"interval": schema.StringAttribute{
				Description:   "Periodic synchronization interval, such as 1h0m0s. Use 0s for manual mirrors.",
				Required:      true,
				PlanModifiers: pushMirrorReplace(),
			},
			"sync_on_commit": schema.BoolAttribute{
				Description: "Whether Forgejo synchronizes the mirror after repository pushes.",
				Required:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"public_key":  schema.StringAttribute{Computed: true},
			"created":     schema.StringAttribute{Computed: true},
			"last_update": schema.StringAttribute{Computed: true},
			"last_error":  schema.StringAttribute{Computed: true},
		},
	}
}

func (r *pushMirrorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureAvatarResource(req, resp)
}

func (r *pushMirrorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data pushMirrorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var password types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("remote_password_wo"), &password)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mirror, response, err := r.client.CreatePushMirror(data.Owner.ValueString(), data.Repository.ValueString(), forgejo.CreatePushMirrorOption{
		RemoteAddress:  data.RemoteAddress.ValueString(),
		RemoteUsername: data.RemoteUsername.ValueString(),
		RemotePassword: password.ValueString(),
		BranchFilter:   data.BranchFilter.ValueString(),
		Interval:       data.Interval.ValueString(),
		SyncONCommit:   data.SyncOnCommit.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Forgejo push mirror", forgejoAPIError(response, err))
		return
	}
	data.from(mirror)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *pushMirrorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data pushMirrorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mirror, response, err := r.client.GetPushMirror(data.Owner.ValueString(), data.Repository.ValueString(), data.RemoteName.ValueString())
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forgejo push mirror", forgejoAPIError(response, err))
		return
	}
	data.from(mirror)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *pushMirrorResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	// Every configurable push-mirror field requires replacement.
}

func (r *pushMirrorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data pushMirrorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.DeletePushMirror(data.Owner.ValueString(), data.Repository.ValueString(), data.RemoteName.ValueString())
	if err != nil && (response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete Forgejo push mirror", forgejoAPIError(response, err))
	}
}

func (r *pushMirrorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Unable to parse import identifier", fmt.Sprintf("Expected 'owner/repository/remote_name', got %q", req.ID))
		return
	}
	mirror, response, err := r.client.GetPushMirror(parts[0], parts[1], parts[2])
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Forgejo push mirror", forgejoAPIError(response, err))
		return
	}
	data := pushMirrorResourceModel{
		Owner:      types.StringValue(parts[0]),
		Repository: types.StringValue(parts[1]),
	}
	data.from(mirror)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func NewPushMirrorResource() resource.Resource {
	return &pushMirrorResource{}
}
