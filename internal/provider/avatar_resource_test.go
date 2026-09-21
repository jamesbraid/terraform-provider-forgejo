package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAvatarHash(t *testing.T) {
	hash, err := avatarHash(3, "aW1hZ2U=")
	require.NoError(t, err)
	require.Equal(t, "35ca5cbe654bc0987dabb0615d16601467754b0f258afcdb2d65a859f4843d00", hash)
}

func TestAvatarHashRejectsInvalidBase64(t *testing.T) {
	_, err := avatarHash(3, "not base64")
	require.Error(t, err)
}

func TestAvatarIdentifier(t *testing.T) {
	require.Equal(t, "abc123", avatarIdentifier("https://forge.example/avatars/abc123?size=870"))
	require.Equal(t, "", avatarIdentifier(""))
}

func TestAvatarResourcesConstruct(t *testing.T) {
	require.NotNil(t, NewUserAvatarResource())
	require.NotNil(t, NewOrganizationAvatarResource())
}
