package provider

import (
	"context"
	"fmt"
	"strconv"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.Resource                = &oauth2AuthSourceResource{}
	_ resource.ResourceWithConfigure   = &oauth2AuthSourceResource{}
	_ resource.ResourceWithImportState = &oauth2AuthSourceResource{}
)

type oauth2AuthSourceResource struct {
	client *forgejo.Client
}

type oauth2AuthSourceURLMappingModel struct {
	AuthURL    types.String `tfsdk:"auth_url"`
	TokenURL   types.String `tfsdk:"token_url"`
	ProfileURL types.String `tfsdk:"profile_url"`
	EmailURL   types.String `tfsdk:"email_url"`
	Tenant     types.String `tfsdk:"tenant"`
}

var oauth2AuthSourceURLMappingTypes = map[string]attr.Type{
	"auth_url": types.StringType, "token_url": types.StringType,
	"profile_url": types.StringType, "email_url": types.StringType,
	"tenant": types.StringType,
}

type oauth2AuthSourceResourceModel struct {
	ID                            types.Int64  `tfsdk:"id"`
	Name                          types.String `tfsdk:"name"`
	IsActive                      types.Bool   `tfsdk:"is_active"`
	Provider                      types.String `tfsdk:"oauth2_provider"`
	ClientID                      types.String `tfsdk:"client_id"`
	OpenIDConnectAutoDiscoveryURL types.String `tfsdk:"openid_connect_auto_discovery_url"`
	Scopes                        types.List   `tfsdk:"scopes"`
	IconURL                       types.String `tfsdk:"icon_url"`
	CustomURLMapping              types.Object `tfsdk:"custom_url_mapping"`
	AttributeSSHPublicKey         types.String `tfsdk:"attribute_ssh_public_key"`
	RequiredClaimName             types.String `tfsdk:"required_claim_name"`
	RequiredClaimValue            types.String `tfsdk:"required_claim_value"`
	GroupClaimName                types.String `tfsdk:"group_claim_name"`
	AdminGroup                    types.String `tfsdk:"admin_group"`
	GroupTeamMap                  types.String `tfsdk:"group_team_map"`
	GroupTeamMapRemoval           types.Bool   `tfsdk:"group_team_map_removal"`
	DynGroupMaps                  types.String `tfsdk:"dyn_group_maps"`
	DynGroupMapsRemoval           types.Bool   `tfsdk:"dyn_group_maps_removal"`
	QuotaGroupClaimName           types.String `tfsdk:"quota_group_claim_name"`
	QuotaGroupMap                 types.String `tfsdk:"quota_group_map"`
	QuotaGroupMapRemoval          types.Bool   `tfsdk:"quota_group_map_removal"`
	RestrictedGroup               types.String `tfsdk:"restricted_group"`
	SkipLocalTwoFA                types.Bool   `tfsdk:"skip_local_two_fa"`
	AllowUsernameChange           types.Bool   `tfsdk:"allow_username_change"`
	ClientSecretWO                types.String `tfsdk:"client_secret_wo"`
	ClientSecretWOVersion         types.Int64  `tfsdk:"client_secret_wo_version"`
}

func (m *oauth2AuthSourceResourceModel) from(ctx context.Context, source *models.OAuth2AuthSource) diag.Diagnostics {
	m.ID = types.Int64Value(source.ID)
	m.Name = types.StringValue(source.Name)
	m.IsActive = types.BoolValue(source.IsActive)
	m.Provider = types.StringValue(source.Provider)
	m.ClientID = types.StringValue(source.ClientID)
	m.OpenIDConnectAutoDiscoveryURL = types.StringValue(source.OpenIDConnectAutoDiscoveryURL)
	m.IconURL = types.StringValue(source.IconURL)
	m.AttributeSSHPublicKey = types.StringValue(source.AttributeSSHPublicKey)
	m.RequiredClaimName = types.StringValue(source.RequiredClaimName)
	m.RequiredClaimValue = types.StringValue(source.RequiredClaimValue)
	m.GroupClaimName = types.StringValue(source.GroupClaimName)
	m.AdminGroup = types.StringValue(source.AdminGroup)
	m.GroupTeamMap = types.StringValue(source.GroupTeamMap)
	m.GroupTeamMapRemoval = types.BoolValue(source.GroupTeamMapRemoval)
	m.DynGroupMaps = types.StringValue(source.DynGroupMaps)
	m.DynGroupMapsRemoval = types.BoolValue(source.DynGroupMapsRemoval)
	m.QuotaGroupClaimName = types.StringValue(source.QuotaGroupClaimName)
	m.QuotaGroupMap = types.StringValue(source.QuotaGroupMap)
	m.QuotaGroupMapRemoval = types.BoolValue(source.QuotaGroupMapRemoval)
	m.RestrictedGroup = types.StringValue(source.RestrictedGroup)
	m.SkipLocalTwoFA = types.BoolValue(source.SkipLocalTwoFA)
	m.AllowUsernameChange = types.BoolValue(source.AllowUsernameChange)
	var diags diag.Diagnostics
	m.Scopes, diags = types.ListValueFrom(ctx, types.StringType, source.Scopes)
	if source.CustomURLMapping == nil {
		m.CustomURLMapping = types.ObjectNull(oauth2AuthSourceURLMappingTypes)
	} else {
		mapping := oauth2AuthSourceURLMappingModel{
			AuthURL:    types.StringValue(source.CustomURLMapping.AuthURL),
			TokenURL:   types.StringValue(source.CustomURLMapping.TokenURL),
			ProfileURL: types.StringValue(source.CustomURLMapping.ProfileURL),
			EmailURL:   types.StringValue(source.CustomURLMapping.EmailURL),
			Tenant:     types.StringValue(source.CustomURLMapping.Tenant),
		}
		value, d := types.ObjectValueFrom(ctx, oauth2AuthSourceURLMappingTypes, mapping)
		diags.Append(d...)
		m.CustomURLMapping = value
	}
	return diags
}

func (m oauth2AuthSourceResourceModel) option(ctx context.Context, secret string) (models.OAuth2AuthSourceOptions, diag.Diagnostics) {
	var scopes []string
	var diags diag.Diagnostics
	if !m.Scopes.IsNull() && !m.Scopes.IsUnknown() {
		diags.Append(m.Scopes.ElementsAs(ctx, &scopes, false)...)
	}
	source := models.OAuth2AuthSource{
		ID: m.ID.ValueInt64(), Name: m.Name.ValueString(), IsActive: m.IsActive.ValueBool(),
		Provider: m.Provider.ValueString(), ClientID: m.ClientID.ValueString(),
		OpenIDConnectAutoDiscoveryURL: m.OpenIDConnectAutoDiscoveryURL.ValueString(),
		Scopes:                        scopes, IconURL: m.IconURL.ValueString(),
		AttributeSSHPublicKey: m.AttributeSSHPublicKey.ValueString(),
		RequiredClaimName:     m.RequiredClaimName.ValueString(), RequiredClaimValue: m.RequiredClaimValue.ValueString(),
		GroupClaimName: m.GroupClaimName.ValueString(), AdminGroup: m.AdminGroup.ValueString(),
		GroupTeamMap: m.GroupTeamMap.ValueString(), GroupTeamMapRemoval: m.GroupTeamMapRemoval.ValueBool(),
		DynGroupMaps: m.DynGroupMaps.ValueString(), DynGroupMapsRemoval: m.DynGroupMapsRemoval.ValueBool(),
		QuotaGroupClaimName: m.QuotaGroupClaimName.ValueString(), QuotaGroupMap: m.QuotaGroupMap.ValueString(),
		QuotaGroupMapRemoval: m.QuotaGroupMapRemoval.ValueBool(),
		RestrictedGroup:      m.RestrictedGroup.ValueString(), SkipLocalTwoFA: m.SkipLocalTwoFA.ValueBool(),
		AllowUsernameChange: m.AllowUsernameChange.ValueBool(),
	}
	if !m.CustomURLMapping.IsNull() && !m.CustomURLMapping.IsUnknown() {
		var mapping oauth2AuthSourceURLMappingModel
		diags.Append(m.CustomURLMapping.As(ctx, &mapping, basetypes.ObjectAsOptions{})...)
		source.CustomURLMapping = &models.OAuth2AuthSourceURLMapping{
			AuthURL: mapping.AuthURL.ValueString(), TokenURL: mapping.TokenURL.ValueString(),
			ProfileURL: mapping.ProfileURL.ValueString(), EmailURL: mapping.EmailURL.ValueString(),
			Tenant: mapping.Tenant.ValueString(),
		}
	}
	return models.OAuth2AuthSourceOptions{Source: source, ClientSecret: secret}, diags
}

// updateOption starts from a fresh API read because PUT resets every omitted
// field. Only HCL-configured fields should be managed by this resource.
func (m oauth2AuthSourceResourceModel) updateOption(ctx context.Context, current models.OAuth2AuthSource, secret string) (models.OAuth2AuthSourceOptions, diag.Diagnostics) {
	current.Name = m.Name.ValueString()
	current.Provider = m.Provider.ValueString()
	current.ClientID = m.ClientID.ValueString()
	if !m.IsActive.IsNull() {
		current.IsActive = m.IsActive.ValueBool()
	}
	if !m.OpenIDConnectAutoDiscoveryURL.IsNull() {
		current.OpenIDConnectAutoDiscoveryURL = m.OpenIDConnectAutoDiscoveryURL.ValueString()
	}
	if !m.IconURL.IsNull() {
		current.IconURL = m.IconURL.ValueString()
	}
	if !m.AttributeSSHPublicKey.IsNull() {
		current.AttributeSSHPublicKey = m.AttributeSSHPublicKey.ValueString()
	}
	if !m.RequiredClaimName.IsNull() {
		current.RequiredClaimName = m.RequiredClaimName.ValueString()
	}
	if !m.RequiredClaimValue.IsNull() {
		current.RequiredClaimValue = m.RequiredClaimValue.ValueString()
	}
	if !m.GroupClaimName.IsNull() {
		current.GroupClaimName = m.GroupClaimName.ValueString()
	}
	if !m.AdminGroup.IsNull() {
		current.AdminGroup = m.AdminGroup.ValueString()
	}
	if !m.GroupTeamMap.IsNull() {
		current.GroupTeamMap = m.GroupTeamMap.ValueString()
	}
	if !m.GroupTeamMapRemoval.IsNull() {
		current.GroupTeamMapRemoval = m.GroupTeamMapRemoval.ValueBool()
	}
	if !m.DynGroupMaps.IsNull() {
		current.DynGroupMaps = m.DynGroupMaps.ValueString()
	}
	if !m.DynGroupMapsRemoval.IsNull() {
		current.DynGroupMapsRemoval = m.DynGroupMapsRemoval.ValueBool()
	}
	if !m.QuotaGroupClaimName.IsNull() {
		current.QuotaGroupClaimName = m.QuotaGroupClaimName.ValueString()
	}
	if !m.QuotaGroupMap.IsNull() {
		current.QuotaGroupMap = m.QuotaGroupMap.ValueString()
	}
	if !m.QuotaGroupMapRemoval.IsNull() {
		current.QuotaGroupMapRemoval = m.QuotaGroupMapRemoval.ValueBool()
	}
	if !m.RestrictedGroup.IsNull() {
		current.RestrictedGroup = m.RestrictedGroup.ValueString()
	}
	if !m.SkipLocalTwoFA.IsNull() {
		current.SkipLocalTwoFA = m.SkipLocalTwoFA.ValueBool()
	}
	if !m.AllowUsernameChange.IsNull() {
		current.AllowUsernameChange = m.AllowUsernameChange.ValueBool()
	}
	var diags diag.Diagnostics
	if !m.Scopes.IsNull() {
		var scopes []string
		diags.Append(m.Scopes.ElementsAs(ctx, &scopes, false)...)
		current.Scopes = scopes
	}
	if !m.CustomURLMapping.IsNull() {
		var mapping oauth2AuthSourceURLMappingModel
		diags.Append(m.CustomURLMapping.As(ctx, &mapping, basetypes.ObjectAsOptions{})...)
		if current.CustomURLMapping == nil {
			current.CustomURLMapping = &models.OAuth2AuthSourceURLMapping{}
		}
		if !mapping.AuthURL.IsNull() {
			current.CustomURLMapping.AuthURL = mapping.AuthURL.ValueString()
		}
		if !mapping.TokenURL.IsNull() {
			current.CustomURLMapping.TokenURL = mapping.TokenURL.ValueString()
		}
		if !mapping.ProfileURL.IsNull() {
			current.CustomURLMapping.ProfileURL = mapping.ProfileURL.ValueString()
		}
		if !mapping.EmailURL.IsNull() {
			current.CustomURLMapping.EmailURL = mapping.EmailURL.ValueString()
		}
		if !mapping.Tenant.IsNull() {
			current.CustomURLMapping.Tenant = mapping.Tenant.ValueString()
		}
	}
	return models.OAuth2AuthSourceOptions{Source: current, ClientSecret: secret}, diags
}

func (r *oauth2AuthSourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oauth2_auth_source"
}

func (r *oauth2AuthSourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optionalString := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Description: description, Optional: true, Computed: true}
	}
	optionalBool := func(description string) schema.BoolAttribute {
		return schema.BoolAttribute{Description: description, Optional: true, Computed: true}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Forgejo administrator-managed OAuth2 login source. Requires an administrator token and Forgejo's auth-source API.",
		Attributes: map[string]schema.Attribute{
			"id":                                schema.Int64Attribute{Description: "Numeric source identifier, also used for import.", Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"name":                              schema.StringAttribute{Description: "Login source name.", Required: true},
			"is_active":                         optionalBool("Whether this source is available for sign-in."),
			"oauth2_provider":                   schema.StringAttribute{Description: "Forgejo OAuth2 provider name, such as openidConnect.", Required: true},
			"client_id":                         schema.StringAttribute{Description: "OAuth2 client identifier.", Required: true},
			"openid_connect_auto_discovery_url": optionalString("OpenID Connect discovery URL."),
			"scopes":                            schema.ListAttribute{Description: "OAuth2 scopes in request order.", ElementType: types.StringType, Optional: true, Computed: true},
			"icon_url":                          optionalString("Sign-in icon URL."),
			"custom_url_mapping": schema.SingleNestedAttribute{
				Description: "Custom OAuth2 provider endpoints.", Optional: true, Computed: true,
				Attributes: map[string]schema.Attribute{
					"auth_url":    optionalString("Authorization endpoint."),
					"token_url":   optionalString("Token endpoint."),
					"profile_url": optionalString("Profile endpoint."),
					"email_url":   optionalString("Email endpoint."),
					"tenant":      optionalString("Provider tenant."),
				},
			},
			"attribute_ssh_public_key": optionalString("OAuth2 attribute containing SSH public keys."),
			"required_claim_name":      optionalString("Required claim name."),
			"required_claim_value":     optionalString("Required claim value."),
			"group_claim_name":         optionalString("Group claim name."),
			"admin_group":              optionalString("Administrator group."),
			"group_team_map":           optionalString("Group-to-team mapping."),
			"group_team_map_removal":   optionalBool("Remove team memberships absent from group mapping."),
			"dyn_group_maps":           optionalString("Dynamic group maps."),
			"dyn_group_maps_removal":   optionalBool("Remove memberships absent from dynamic group maps."),
			"quota_group_claim_name":   optionalString("Quota group claim name."),
			"quota_group_map":          optionalString("Quota group map."),
			"quota_group_map_removal":  optionalBool("Remove quota group assignments absent from mapping."),
			"restricted_group":         optionalString("Restricted user group."),
			"skip_local_two_fa":        optionalBool("Skip Forgejo's local two-factor check."),
			"allow_username_change":    optionalBool("Allow users to change their username."),
			"client_secret_wo":         schema.StringAttribute{Description: "Write-only OAuth2 client secret. Required when creating a source; omit when importing. Set client_secret_wo_version to rotate it later.", Optional: true, Sensitive: true, WriteOnly: true},
			"client_secret_wo_version": schema.Int64Attribute{Description: "Change this revision to replace the client secret. Omit on import to preserve the existing secret.", Optional: true},
		},
	}
}

func (r *oauth2AuthSourceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func oauth2AuthSourceSecret(ctx context.Context, config tfsdk.Config) (string, diag.Diagnostics) {
	var secret types.String
	diags := config.GetAttribute(ctx, path.Root("client_secret_wo"), &secret)
	if secret.IsNull() || secret.IsUnknown() {
		return "", diags
	}
	return secret.ValueString(), diags
}

func oauth2AuthSourceAPIError(response forgejo.Response, err error) string {
	if response.Response == nil {
		return forgejoAPIError(nil, err)
	}
	return forgejoAPIError(&response, err)
}

func (r *oauth2AuthSourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data oauth2AuthSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	secret, diags := oauth2AuthSourceSecret(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if secret == "" {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret_wo"), "Missing OAuth2 client secret", "Creating a login source requires a nonempty client_secret_wo.")
		return
	}
	option, diags := data.option(ctx, secret)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	source, response, err := r.client.CreateAdminOAuth2AuthSource(ctx, option)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create OAuth2 auth source", oauth2AuthSourceAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, &source)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *oauth2AuthSourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data oauth2AuthSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	source, response, err := r.client.GetAdminOAuth2AuthSource(ctx, data.ID.ValueInt64())
	if err != nil {
		if response.Response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read OAuth2 auth source", oauth2AuthSourceAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, &source)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *oauth2AuthSourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, prior, configured oauth2AuthSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &configured)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secret := ""
	if !data.ClientSecretWOVersion.Equal(prior.ClientSecretWOVersion) {
		var diags diag.Diagnostics
		secret, diags = oauth2AuthSourceSecret(ctx, req.Config)
		resp.Diagnostics.Append(diags...)
		if secret == "" {
			resp.Diagnostics.AddAttributeError(path.Root("client_secret_wo"), "Missing OAuth2 client secret", "Changing client_secret_wo_version requires a nonempty client_secret_wo.")
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	current, response, err := r.client.GetAdminOAuth2AuthSource(ctx, data.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read OAuth2 auth source before update", oauth2AuthSourceAPIError(response, err))
		return
	}
	option, diags := configured.updateOption(ctx, current, secret)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	source, response, err := r.client.UpdateAdminOAuth2AuthSource(ctx, data.ID.ValueInt64(), option)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update OAuth2 auth source", oauth2AuthSourceAPIError(response, err))
		return
	}
	resp.Diagnostics.Append(data.from(ctx, &source)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *oauth2AuthSourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data oauth2AuthSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.DeleteAdminOAuth2AuthSource(ctx, data.ID.ValueInt64())
	if err != nil && (response.Response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete OAuth2 auth source", oauth2AuthSourceAPIError(response, err))
	}
}

func (r *oauth2AuthSourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil || id <= 0 {
		resp.Diagnostics.AddError("Invalid OAuth2 auth source ID", "Import with a positive numeric source ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func NewOAuth2AuthSourceResource() resource.Resource { return &oauth2AuthSourceResource{} }
