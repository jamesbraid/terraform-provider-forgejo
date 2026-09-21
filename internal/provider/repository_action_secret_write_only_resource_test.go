package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccRepositoryActionSecretResourceWriteOnly(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_repository" "write_only" {
	name = "write_only_secret"
}
resource "forgejo_repository_action_secret" "write_only" {
	repository_id   = forgejo_repository.write_only.id
	name            = "WRITE_ONLY_SECRET"
	data_wo         = "initial-secret-value"
	data_wo_version = 1
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_repository_action_secret.write_only", plancheck.ResourceActionCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("forgejo_repository_action_secret.write_only", "data_wo"),
					resource.TestCheckResourceAttr("forgejo_repository_action_secret.write_only", "data_wo_version", "1"),
				),
			},
			{
				Config: providerConfig + `
resource "forgejo_repository" "write_only" {
	name = "write_only_secret"
}
resource "forgejo_repository_action_secret" "write_only" {
	repository_id   = forgejo_repository.write_only.id
	name            = "WRITE_ONLY_SECRET"
	data_wo         = "ignored-without-version-change"
	data_wo_version = 1
}`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config: providerConfig + `
resource "forgejo_repository" "write_only" {
	name = "write_only_secret"
}
resource "forgejo_repository_action_secret" "write_only" {
	repository_id   = forgejo_repository.write_only.id
	name            = "WRITE_ONLY_SECRET"
	data_wo         = "rotated-secret-value"
	data_wo_version = 2
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_repository_action_secret.write_only", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("forgejo_repository_action_secret.write_only", "data_wo"),
					resource.TestCheckResourceAttr("forgejo_repository_action_secret.write_only", "data_wo_version", "2"),
				),
			},
			{
				ResourceName:                         "forgejo_repository_action_secret.write_only",
				ImportState:                          true,
				ImportStatePersist:                   true,
				ImportStateIdFunc:                    actionSecretImportStateID("forgejo_repository_action_secret.write_only", "repository_id"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "repository_id",
				ImportStateVerifyIgnore:              []string{"data_wo_version"},
			},
			{
				Config: providerConfig + `
resource "forgejo_repository" "write_only" {
	name = "write_only_secret"
}
resource "forgejo_repository_action_secret" "write_only" {
	repository_id = forgejo_repository.write_only.id
	name          = "WRITE_ONLY_SECRET"
	data_wo       = "adopted-existing-value"
}`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config: providerConfig + `
resource "forgejo_repository" "write_only" {
	name = "write_only_secret"
}
resource "forgejo_repository_action_secret" "write_only" {
	repository_id   = forgejo_repository.write_only.id
	name            = "WRITE_ONLY_SECRET"
	data_wo         = "published-after-adoption"
	data_wo_version = 3
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_repository_action_secret.write_only", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("forgejo_repository_action_secret.write_only", "data_wo_version", "3"),
			},
		},
	})
}

func actionSecretImportStateID(resourceName, parentAttribute string) resource.ImportStateIdFunc {
	return func(state *terraform.State) (string, error) {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", resourceName)
		}

		return resourceState.Primary.Attributes[parentAttribute] + "/" + resourceState.Primary.Attributes["name"], nil
	}
}
