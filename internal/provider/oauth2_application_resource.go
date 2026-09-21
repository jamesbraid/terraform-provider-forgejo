package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var (
	_ resource.Resource                = &oauth2ApplicationResource{}
	_ resource.ResourceWithConfigure   = &oauth2ApplicationResource{}
	_ resource.ResourceWithImportState = &oauth2ApplicationResource{}
)

type oauth2ApplicationResource struct {
	client *forgejo.Client
}

type oauth2ApplicationResourceModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	ClientID           types.String `tfsdk:"client_id"`
	ClientSecret       types.String `tfsdk:"client_secret"`
	RedirectURIs       types.List   `tfsdk:"redirect_uris"`
	ConfidentialClient types.Bool   `tfsdk:"confidential_client"`
	CreatedAt          types.String `tfsdk:"created_at"`
}

func (m *oauth2ApplicationResourceModel) from(ctx context.Context, app *forgejo.Oauth2) diag.Diagnostics {
	if app == nil {
		return nil
	}

	m.ID = types.Int64Value(app.ID)
	m.Name = types.StringValue(app.Name)
	m.ClientID = types.StringValue(app.ClientID)
	if app.ClientSecret != "" {
		m.ClientSecret = types.StringValue(app.ClientSecret)
	}
	var diags diag.Diagnostics
	m.RedirectURIs, diags = types.ListValueFrom(ctx, types.StringType, app.RedirectURIs)
	m.ConfidentialClient = types.BoolValue(app.ConfidentialClient)
	m.CreatedAt = types.StringValue(app.Created.Format(time.RFC3339))
	return diags
}

func (m oauth2ApplicationResourceModel) option(ctx context.Context) (forgejo.CreateOauth2Option, diag.Diagnostics) {
	var redirectURIs []string
	diags := m.RedirectURIs.ElementsAs(ctx, &redirectURIs, false)
	return forgejo.CreateOauth2Option{
		Name:               m.Name.ValueString(),
		ConfidentialClient: m.ConfidentialClient.ValueBool(),
		RedirectURIs:       redirectURIs,
	}, diags
}

func (r *oauth2ApplicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth2_application"
}

func (r *oauth2ApplicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Forgejo OAuth2 application resource. Applications belong to the user configured on the selected provider.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "Numeric identifier of the OAuth2 application.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the OAuth2 application.",
				Required:    true,
			},
			"client_id": schema.StringAttribute{
				Description: "OAuth2 client identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_secret": schema.StringAttribute{
				Description: "OAuth2 client secret. Forgejo returns it only after create or update, so imported applications leave it null.",
				Computed:    true,
				Sensitive:   true,
			},
			"redirect_uris": schema.ListAttribute{
				Description: "Allowed OAuth2 redirect URIs.",
				ElementType: types.StringType,
				Required:    true,
			},
			"confidential_client": schema.BoolAttribute{
				Description: "Whether the application can keep its client secret confidential.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"created_at": schema.StringAttribute{
				Description: "Time at which the OAuth2 application was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *oauth2ApplicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *oauth2ApplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer un(trace(ctx, "Create OAuth2 application resource"))
	var data oauth2ApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	option, diags := data.option(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	app, response, err := r.client.CreateOauth2(option)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create OAuth2 application", forgejoAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, app)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *oauth2ApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer un(trace(ctx, "Read OAuth2 application resource"))
	var data oauth2ApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	app, response, err := r.client.GetOauth2(data.ID.ValueInt64())
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OAuth2 application", forgejoAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, app)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *oauth2ApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer un(trace(ctx, "Update OAuth2 application resource"))
	var data oauth2ApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	option, diags := data.option(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	app, response, err := r.client.UpdateOauth2(data.ID.ValueInt64(), option)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update OAuth2 application", forgejoAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, app)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *oauth2ApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer un(trace(ctx, "Delete OAuth2 application resource"))
	var data oauth2ApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.DeleteOauth2(data.ID.ValueInt64())
	if err != nil && (response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete OAuth2 application", forgejoAPIError(response, err))
	}
}

func (r *oauth2ApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer un(trace(ctx, "Import OAuth2 application resource"))
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Unable to parse import identifier", fmt.Sprintf("Failed to parse OAuth2 application ID: %s", err))
		return
	}
	app, response, err := r.client.GetOauth2(id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read OAuth2 application", forgejoAPIError(response, err))
		return
	}
	data := oauth2ApplicationResourceModel{ClientSecret: types.StringNull()}
	resp.Diagnostics.Append(data.from(ctx, app)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func NewOAuth2ApplicationResource() resource.Resource {
	return &oauth2ApplicationResource{}
}
