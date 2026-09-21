package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOrganizationWebhookResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_organization" "test" {
	name = "webhook-org"
}
resource "forgejo_organization_webhook" "test" {
	organization = forgejo_organization.test.name
	type         = "forgejo"
	active       = true
	events       = ["issue_comment", "pull_request"]
	config = {
		content_type = "json"
		url          = "http://example.com/organization"
		secret       = "organization-secret"
	}
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_organization_webhook.test", tfjsonpath.New("organization"), knownvalue.StringExact("webhook-org")),
					statecheck.ExpectKnownValue("forgejo_organization_webhook.test", tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_organization_webhook.test", tfjsonpath.New("config").AtMapKey("secret"), knownvalue.StringExact("organization-secret")),
				},
			},
			{
				ResourceName:                         "forgejo_organization_webhook.test",
				ImportState:                          true,
				ImportStateIdFunc:                    webhookImportID("forgejo_organization_webhook.test", "webhook-org/"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "webhook_id",
				ImportStateVerifyIgnore:              []string{"config.secret", "config.%"},
			},
			{
				ResourceName:  "forgejo_organization_webhook.test",
				ImportState:   true,
				ImportStateId: "invalid",
				ExpectError:   regexp.MustCompile("Expected import identifier with format: 'organization/webhookID'"),
			},
		},
	})
}

func webhookImportID(resourceName, prefix string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("not found: %s", resourceName)
		}

		return prefix + rs.Primary.Attributes["webhook_id"], nil
	}
}
