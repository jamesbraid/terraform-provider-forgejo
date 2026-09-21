package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func avatarHash(ownerID int64, image string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(image)
	if err != nil {
		return "", fmt.Errorf("decode avatar image: %w", err)
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(strconv.FormatInt(ownerID, 10)))
	_, _ = hash.Write([]byte{'-'})
	_, _ = hash.Write(data)
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func avatarIdentifier(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Path == "" {
		return ""
	}
	identifier := path.Base(parsed.Path)
	if identifier == "." || identifier == "/" {
		return ""
	}
	return identifier
}

func avatarSchema(ownerName, ownerDescription string) schema.Schema {
	return schema.Schema{
		MarkdownDescription: fmt.Sprintf("Manages the avatar for an existing Forgejo %s.", ownerDescription),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource identifier.",
				Computed:    true,
			},
			ownerName: schema.StringAttribute{
				Description: fmt.Sprintf("Name of the %s whose avatar is managed.", ownerDescription),
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image": schema.StringAttribute{
				Description: "Base64-encoded avatar image.",
				Required:    true,
			},
			"hash": schema.StringAttribute{
				Description: "Forgejo content identifier for the stored avatar.",
				Computed:    true,
			},
		},
	}
}

func configureAvatarResource(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *forgejo.Client {
	if req.ProviderData == nil {
		return nil
	}
	client, ok := req.ProviderData.(*forgejo.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *forgejo.Client, got: %T", req.ProviderData))
		return nil
	}
	return client
}

type userAvatarResource struct {
	client *forgejo.Client
}

type userAvatarResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Username types.String `tfsdk:"username"`
	Image    types.String `tfsdk:"image"`
	Hash     types.String `tfsdk:"hash"`
}

var (
	_ resource.Resource              = &userAvatarResource{}
	_ resource.ResourceWithConfigure = &userAvatarResource{}
)

func (r *userAvatarResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_avatar"
}

func (r *userAvatarResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = avatarSchema("username", "user")
}

func (r *userAvatarResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureAvatarResource(req, resp)
}

func (r *userAvatarResource) reconcile(data *userAvatarResourceModel, diagnostics *diag.Diagnostics) bool {
	user, response, err := r.client.GetUserInfo(data.Username.ValueString())
	if err != nil {
		diagnostics.AddError("Unable to read Forgejo user", forgejoAPIError(response, err))
		return false
	}
	desiredHash, err := avatarHash(user.ID, data.Image.ValueString())
	if err != nil {
		diagnostics.AddError("Invalid avatar image", err.Error())
		return false
	}
	if avatarIdentifier(user.AvatarURL) != desiredHash {
		response, err = r.client.UpdateUserAvatarFor(data.Username.ValueString(), forgejo.UpdateUserAvatarOption{Image: data.Image.ValueString()})
		if err != nil {
			diagnostics.AddError("Unable to update Forgejo user avatar", forgejoAPIError(response, err))
			return false
		}
	}
	data.ID = data.Username
	data.Hash = types.StringValue(desiredHash)
	return true
}

func (r *userAvatarResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data userAvatarResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !r.reconcile(&data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userAvatarResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data userAvatarResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	user, response, err := r.client.GetUserInfo(data.Username.ValueString())
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forgejo user", forgejoAPIError(response, err))
		return
	}
	desiredHash, err := avatarHash(user.ID, data.Image.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid avatar image", err.Error())
		return
	}
	if avatarIdentifier(user.AvatarURL) != desiredHash {
		resp.State.RemoveResource(ctx)
		return
	}
	data.Hash = types.StringValue(desiredHash)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userAvatarResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data userAvatarResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !r.reconcile(&data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *userAvatarResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data userAvatarResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.DeleteUserAvatarFor(data.Username.ValueString())
	if err != nil && (response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete Forgejo user avatar", forgejoAPIError(response, err))
	}
}

func NewUserAvatarResource() resource.Resource {
	return &userAvatarResource{}
}

type organizationAvatarResource struct {
	client *forgejo.Client
}

type organizationAvatarResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Organization types.String `tfsdk:"organization"`
	Image        types.String `tfsdk:"image"`
	Hash         types.String `tfsdk:"hash"`
}

var (
	_ resource.Resource              = &organizationAvatarResource{}
	_ resource.ResourceWithConfigure = &organizationAvatarResource{}
)

func (r *organizationAvatarResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_avatar"
}

func (r *organizationAvatarResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = avatarSchema("organization", "organization")
}

func (r *organizationAvatarResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureAvatarResource(req, resp)
}

func (r *organizationAvatarResource) reconcile(data *organizationAvatarResourceModel, diagnostics *diag.Diagnostics) bool {
	organization, response, err := r.client.GetOrg(data.Organization.ValueString())
	if err != nil {
		diagnostics.AddError("Unable to read Forgejo organization", forgejoAPIError(response, err))
		return false
	}
	desiredHash, err := avatarHash(organization.ID, data.Image.ValueString())
	if err != nil {
		diagnostics.AddError("Invalid avatar image", err.Error())
		return false
	}
	if avatarIdentifier(organization.AvatarURL) != desiredHash {
		response, err = r.client.UpdateOrgAvatar(data.Organization.ValueString(), forgejo.UpdateUserAvatarOption{Image: data.Image.ValueString()})
		if err != nil {
			diagnostics.AddError("Unable to update Forgejo organization avatar", forgejoAPIError(response, err))
			return false
		}
	}
	data.ID = data.Organization
	data.Hash = types.StringValue(desiredHash)
	return true
}

func (r *organizationAvatarResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data organizationAvatarResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !r.reconcile(&data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationAvatarResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data organizationAvatarResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	organization, response, err := r.client.GetOrg(data.Organization.ValueString())
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Forgejo organization", forgejoAPIError(response, err))
		return
	}
	desiredHash, err := avatarHash(organization.ID, data.Image.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid avatar image", err.Error())
		return
	}
	if avatarIdentifier(organization.AvatarURL) != desiredHash {
		resp.State.RemoveResource(ctx)
		return
	}
	data.Hash = types.StringValue(desiredHash)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationAvatarResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data organizationAvatarResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !r.reconcile(&data, &resp.Diagnostics) {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationAvatarResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data organizationAvatarResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := r.client.DeleteOrgAvatar(data.Organization.ValueString())
	if err != nil && (response == nil || response.StatusCode != 404) {
		resp.Diagnostics.AddError("Unable to delete Forgejo organization avatar", forgejoAPIError(response, err))
	}
}

func NewOrganizationAvatarResource() resource.Resource {
	return &organizationAvatarResource{}
}
