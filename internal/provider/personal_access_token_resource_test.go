package provider_test

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stretchr/testify/require"
)

func TestAccPersonalAccessTokenResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing (non-existent user)
			{
				Config: providerConfig + `
resource "forgejo_personal_access_token" "test" {
	user   = "non_existing_user"
	name   = "tftest"
	scopes = ["all"]
}`,
				ExpectError: regexp.MustCompile("Personal access token for user \"non_existing_user\" not found"),
			},
			// Create and Read testing
			{
				Config: providerConfig + providerBasicAuthConfig + `
resource "forgejo_user" "test" {
	login    = "test_user"
	password = "password"
	email    = "test_user@example.com"
}
resource "forgejo_personal_access_token" "test" {
	provider = forgejo.basicAuth

	user   = forgejo_user.test.login
	name   = "tftest"
	scopes = ["all"]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_personal_access_token.test", plancheck.ResourceActionCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("tftest")),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("token_last_eight"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("all"),
					})),
				},
			},
			// Duplicate token names are not allowed.
			{
				Config: providerConfig + providerBasicAuthConfig + `
resource "forgejo_user" "test" {
	login    = "test_user"
	password = "password"
	email    = "test_user@example.com"
}
resource "forgejo_personal_access_token" "test" {
	provider = forgejo.basicAuth

	user   = forgejo_user.test.login
	name   = "tftest"
	scopes = ["all"]
}
resource "forgejo_personal_access_token" "test1" {
	provider = forgejo.basicAuth

	user   = forgejo_user.test.login
	name   = "tftest"
	scopes = ["all"]
}`,
				ExpectError: regexp.MustCompile("access token name has been used already"),
			},
			// Changing scope recreates the token.
			{
				Config: providerConfig + providerBasicAuthConfig + `
resource "forgejo_user" "test" {
	login    = "test_user"
	password = "password"
	email    = "test_user@example.com"
}
resource "forgejo_personal_access_token" "test" {
	provider = forgejo.basicAuth

	user   = forgejo_user.test.login
	name   = "tftest"
	scopes = [
		"read:organization",
		"read:repository"
	]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_personal_access_token.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("tftest")),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("token_last_eight"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("read:organization"),
						knownvalue.StringExact("read:repository"),
					})),
				},
			},
			// Changing name recreates the token.
			{
				Config: providerConfig + providerBasicAuthConfig + `
resource "forgejo_user" "test" {
	login    = "test_user"
	password = "password"
	email    = "test_user@example.com"
}
resource "forgejo_personal_access_token" "test" {
	provider = forgejo.basicAuth

	user   = forgejo_user.test.login
	name   = "tftest1"
	scopes = [
		"read:organization",
		"read:repository"
	]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_personal_access_token.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("name"), knownvalue.StringExact("tftest1")),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("token"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("token_last_eight"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_personal_access_token.test", tfjsonpath.New("scopes"), knownvalue.SetExact([]knownvalue.Check{
						knownvalue.StringExact("read:organization"),
						knownvalue.StringExact("read:repository"),
					})),
				},
			},
			// Import existing token metadata without attempting to recover the
			// write-only token value returned only at creation time.
			{
				ResourceName:            "forgejo_personal_access_token.test",
				ImportState:             true,
				ImportStateId:           "test_user/tftest1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"token"},
			},
			// Deleting the token outside Terraform removes it from state and
			// recreates it instead of making every subsequent refresh fail.
			{
				PreConfig: func() {
					client, err := forgejo.NewClient(
						forgejoTestHost,
						forgejo.SetBasicAuth(
							os.Getenv("FORGEJO_USERNAME"),
							os.Getenv("FORGEJO_PASSWORD"),
						),
					)
					require.NoError(t, err)
					tokens, _, err := client.ListAccessTokens("test_user", forgejo.ListAccessTokensOptions{})
					require.NoError(t, err)
					idx := slices.IndexFunc(tokens, func(token *forgejo.AccessToken) bool {
						return token.Name == "tftest1"
					})
					require.NotEqual(t, -1, idx)
					_, err = client.DeleteAccessToken("test_user", tokens[idx].ID)
					require.NoError(t, err)
				},
				Config: providerConfig + providerBasicAuthConfig + `
resource "forgejo_user" "test" {
	login    = "test_user"
	password = "password"
	email    = "test_user@example.com"
}
resource "forgejo_personal_access_token" "test" {
	provider = forgejo.basicAuth

	user   = forgejo_user.test.login
	name   = "tftest1"
	scopes = [
		"read:organization",
		"read:repository"
	]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_personal_access_token.test", plancheck.ResourceActionCreate),
					},
				},
			},
		},
	})
}
