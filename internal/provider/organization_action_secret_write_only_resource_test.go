package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccOrganizationActionSecretResourceWriteOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_organization" "write_only" {
	name = "write-only-secrets"
}
resource "forgejo_organization_action_secret" "write_only" {
	organization    = forgejo_organization.write_only.name
	name            = "WRITE_ONLY_SECRET"
	data_wo         = "initial-secret-value"
	data_wo_version = 1
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_organization_action_secret.write_only", plancheck.ResourceActionCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("forgejo_organization_action_secret.write_only", "data_wo"),
					resource.TestCheckResourceAttr("forgejo_organization_action_secret.write_only", "data_wo_version", "1"),
				),
			},
			{
				Config: providerConfig + `
resource "forgejo_organization" "write_only" {
	name = "write-only-secrets"
}
resource "forgejo_organization_action_secret" "write_only" {
	organization    = forgejo_organization.write_only.name
	name            = "WRITE_ONLY_SECRET"
	data_wo         = "rotated-secret-value"
	data_wo_version = 2
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_organization_action_secret.write_only", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("forgejo_organization_action_secret.write_only", "data_wo"),
					resource.TestCheckResourceAttr("forgejo_organization_action_secret.write_only", "data_wo_version", "2"),
				),
			},
			{
				ResourceName:                         "forgejo_organization_action_secret.write_only",
				ImportState:                          true,
				ImportStateIdFunc:                    actionSecretImportStateID("forgejo_organization_action_secret.write_only", "organization"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "organization",
				ImportStateVerifyIgnore:              []string{"data_wo_version", "organization_id"},
			},
		},
	})
}
