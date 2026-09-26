package provider

import (
	"context"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3/models"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/stretchr/testify/require"
)

func TestOAuth2AuthSourceRoundTrip(t *testing.T) {
	ctx := context.Background()
	source := models.OAuth2AuthSource{
		ID: 42, Name: "Rauthy", IsActive: true, Provider: "openidConnect",
		ClientID: "forgejo", OpenIDConnectAutoDiscoveryURL: "https://auth.example/.well-known/openid-configuration",
		Scopes: []string{"openid", "profile", "email"}, IconURL: "https://auth.example/icon.svg",
		CustomURLMapping: &models.OAuth2AuthSourceURLMapping{
			AuthURL: "https://auth.example/authorize", TokenURL: "https://auth.example/token",
			ProfileURL: "https://auth.example/profile", EmailURL: "https://auth.example/email", Tenant: "home",
		},
		AttributeSSHPublicKey: "ssh", RequiredClaimName: "role", RequiredClaimValue: "member",
		GroupClaimName: "groups", AdminGroup: "admins", GroupTeamMap: `{"engineers":"infra"}`,
		GroupTeamMapRemoval: true, DynGroupMaps: `{"users":[]}`, DynGroupMapsRemoval: true,
		QuotaGroupClaimName: "quota", QuotaGroupMap: `{"large":100}`, QuotaGroupMapRemoval: true,
		RestrictedGroup: "guests", SkipLocalTwoFA: true, AllowUsernameChange: true,
	}
	var model oauth2AuthSourceResourceModel
	require.False(t, model.from(ctx, &source).HasError())
	require.True(t, model.ClientSecretWO.IsNull())
	require.True(t, model.ClientSecretWOVersion.IsNull())
	option, diags := model.option(ctx, "")
	require.False(t, diags.HasError())
	require.Equal(t, source, option.Source)
	require.Empty(t, option.ClientSecret)
}

func TestOAuth2AuthSourceSchemaSecretIsWriteOnly(t *testing.T) {
	var response resource.SchemaResponse
	(&oauth2AuthSourceResource{}).Schema(context.Background(), resource.SchemaRequest{}, &response)
	secret, ok := response.Schema.Attributes["client_secret_wo"].(schema.StringAttribute)
	require.True(t, ok)
	require.True(t, secret.WriteOnly)
	require.True(t, secret.Sensitive)
	version := response.Schema.Attributes["client_secret_wo_version"]
	require.NotNil(t, version)
}
