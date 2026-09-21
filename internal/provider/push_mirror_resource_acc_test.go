package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccPushMirrorResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: pushMirrorConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("forgejo_push_mirror.test", tfjsonpath.New("remote_name"), knownvalue.StringRegexp(regexp.MustCompile(`^remote_mirror_`))),
					statecheck.ExpectKnownValue("forgejo_push_mirror.test", tfjsonpath.New("branch_filter"), knownvalue.StringExact("main")),
					statecheck.ExpectKnownValue("forgejo_push_mirror.test", tfjsonpath.New("sync_on_commit"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func pushMirrorConfig() string {
	return fmt.Sprintf(providerConfig+providerBasicAuthConfig+`
resource "forgejo_repository" "push_mirror_source" {
  name      = "tftest-push-mirror-source"
  auto_init = true
}

resource "forgejo_push_mirror" "test" {
  provider                   = forgejo.basicAuth
  owner                      = %q
  repository                 = forgejo_repository.push_mirror_source.name
  remote_address             = "https://example.com/forgejo-push-mirror-test.git"
  remote_username            = ""
  remote_password_wo         = ""
  remote_password_wo_version = 1
  branch_filter              = "main"
  interval                   = "0s"
  sync_on_commit             = false
}
`, forgejoTestUser)
}
