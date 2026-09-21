package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const (
	avatarImageOne = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	avatarImageTwo = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9ZlSoAAAAASUVORK5CYII="
)

func TestAccUserAvatarResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: userAvatarConfig(avatarImageOne),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_user_avatar.test", tfjsonpath.New("username"), knownvalue.StringExact(forgejoTestUser)),
					statecheck.ExpectKnownValue("forgejo_user_avatar.test", tfjsonpath.New("hash"), knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{64}$`))),
				},
			},
			{
				Config: userAvatarConfig(avatarImageTwo),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_user_avatar.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestAccOrganizationAvatarResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: organizationAvatarConfig(avatarImageOne),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_organization_avatar.test", tfjsonpath.New("organization"), knownvalue.StringExact("avatar-test")),
					statecheck.ExpectKnownValue("forgejo_organization_avatar.test", tfjsonpath.New("hash"), knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{64}$`))),
				},
			},
			{
				Config: organizationAvatarConfig(avatarImageTwo),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_organization_avatar.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func userAvatarConfig(image string) string {
	return fmt.Sprintf(providerConfig+providerBasicAuthConfig+`
resource "forgejo_user_avatar" "test" {
	provider = forgejo.basicAuth

	username = %q
	image    = %q
}`, forgejoTestUser, image)
}

func organizationAvatarConfig(image string) string {
	return fmt.Sprintf(providerConfig+`
resource "forgejo_organization" "avatar_test" {
	name = "avatar-test"
}

resource "forgejo_organization_avatar" "test" {
	organization = forgejo_organization.avatar_test.name
	image        = %q
}`, image)
}
