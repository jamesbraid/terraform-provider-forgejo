package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/stretchr/testify/require"
)

func TestPushMirrorSchemaUsesWriteOnlyPassword(t *testing.T) {
	var response resource.SchemaResponse
	NewPushMirrorResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	require.False(t, response.Diagnostics.HasError())

	password, ok := response.Schema.Attributes["remote_password_wo"].(schema.StringAttribute)
	require.True(t, ok)
	require.True(t, password.Required)
	require.True(t, password.Sensitive)
	require.True(t, password.WriteOnly)

	version, ok := response.Schema.Attributes["remote_password_wo_version"].(schema.Int64Attribute)
	require.True(t, ok)
	require.True(t, version.Required)
}
