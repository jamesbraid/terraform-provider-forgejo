package provider_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccProviderSudoWritesRepositorySecretAsOwner(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "forgejo_user" "owner" {
	login    = "secret-owner"
	email    = "secret-owner@localhost.localdomain"
	password = "owner-passw0rd"
}`,
			},
			{
				Config: providerConfig + `
provider "forgejo" {
	alias    = "owner"
	host     = "http://localhost:3000"
	username = "tfadmin"
	password = "provider-test-password"
	api_token = ""
	sudo     = "secret-owner"
}
resource "forgejo_user" "owner" {
	login    = "secret-owner"
	email    = "secret-owner@localhost.localdomain"
	password = "owner-passw0rd"
}
resource "forgejo_repository" "owned" {
	provider = forgejo.owner
	name     = "sudo-secret"
	depends_on = [forgejo_user.owner]
}
resource "forgejo_repository_action_secret" "owned" {
	provider        = forgejo.owner
	repository_id   = forgejo_repository.owned.id
	name            = "SUDO_SECRET"
	data_wo         = "secret-value"
	data_wo_version = 1
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_repository_action_secret.owned", plancheck.ResourceActionCreate),
					},
				},
			},
			{
				Config: providerConfig + `
provider "forgejo" {
	alias     = "owner"
	host      = "http://localhost:3000"
	username  = "tfadmin"
	password  = "provider-test-password"
	api_token = ""
	sudo      = "secret-owner"
}
resource "forgejo_user" "owner" {
	login    = "secret-owner"
	email    = "secret-owner@localhost.localdomain"
	password = "owner-passw0rd"
}`,
			},
		},
	})
}
