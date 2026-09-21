package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOAuth2ApplicationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: oauth2ApplicationConfig("OAuth test", "https://example.com/callback", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("name"), knownvalue.StringExact("OAuth test")),
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("client_id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("client_secret"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("confidential_client"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("redirect_uris"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("https://example.com/callback"),
					})),
				},
			},
			{
				Config: oauth2ApplicationConfig("OAuth test renamed", "https://example.com/updated", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_oauth2_application.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("name"), knownvalue.StringExact("OAuth test renamed")),
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("client_secret"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("forgejo_oauth2_application.test", tfjsonpath.New("confidential_client"), knownvalue.Bool(false)),
				},
			},
			{
				ResourceName:                         "forgejo_oauth2_application.test",
				ImportState:                          true,
				ImportStateIdFunc:                    oauth2ApplicationImportID,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				ImportStateVerifyIgnore:              []string{"client_secret"},
			},
			{
				ResourceName:  "forgejo_oauth2_application.test",
				ImportState:   true,
				ImportStateId: "invalid",
				ExpectError:   regexp.MustCompile("Failed to parse OAuth2 application ID"),
			},
		},
	})
}

func oauth2ApplicationConfig(name, redirectURI string, confidential bool) string {
	return fmt.Sprintf(providerConfig+providerBasicAuthConfig+`
resource "forgejo_oauth2_application" "test" {
	provider = forgejo.basicAuth

	name                = %q
	redirect_uris       = [%q]
	confidential_client = %t
}`, name, redirectURI, confidential)
}

func oauth2ApplicationImportID(s *terraform.State) (string, error) {
	rs, ok := s.RootModule().Resources["forgejo_oauth2_application.test"]
	if !ok {
		return "", fmt.Errorf("OAuth2 application resource not found")
	}
	return rs.Primary.Attributes["id"], nil
}
