package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccUserWebhookResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_user_webhook" "test" {
	type   = "forgejo"
	active = true
	events = ["issue_comment", "pull_request"]
	config = {
		content_type = "json"
		url          = "http://example.com/user"
		secret       = "user-secret"
	}
}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_user_webhook.test", tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("forgejo_user_webhook.test", tfjsonpath.New("config").AtMapKey("secret"), knownvalue.StringExact("user-secret")),
				},
			},
			{
				ResourceName:                         "forgejo_user_webhook.test",
				ImportState:                          true,
				ImportStateIdFunc:                    webhookImportID("forgejo_user_webhook.test", ""),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "webhook_id",
				ImportStateVerifyIgnore:              []string{"config.secret", "config.%"},
			},
			{
				ResourceName:  "forgejo_user_webhook.test",
				ImportState:   true,
				ImportStateId: "invalid",
				ExpectError:   regexp.MustCompile("Failed to parse webhook ID"),
			},
		},
	})
}
