package provider_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
)

func TestAccRepositoryActionSecretResourceWriteOnlyAdoption(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	testAccPreCheck(t)

	name := fmt.Sprintf("write-only-adoption-%d", time.Now().UnixNano())
	client := actionSecretAdoptionClient(t)
	repository, _, err := client.CreateRepo(forgejo.CreateRepoOption{Name: name})
	require.NoError(t, err)
	repositoryID := repository.ID
	t.Cleanup(func() {
		if _, err := client.DeleteRepo(forgejoTestUser, name); err != nil {
			t.Errorf("delete adoption repository: %v", err)
		}
	})
	_, err = client.CreateRepoActionSecret(forgejoTestUser, name, forgejo.CreateSecretOption{
		Name: "WRITE_ONLY_ADOPTED",
		Data: "existing-value",
	})
	require.NoError(t, err)

	config := func(version string) string {
		return providerConfig + fmt.Sprintf(`
resource "forgejo_repository_action_secret" "adopted" {
	repository_id   = %d
	name            = "WRITE_ONLY_ADOPTED"
	data_wo         = "adopted-value"
	%s
}`, repositoryID, version)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName:       "forgejo_repository_action_secret.adopted",
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateId:      fmt.Sprintf("%d/WRITE_ONLY_ADOPTED", repositoryID),
				ImportStateCheck:   actionSecretAdoptionImportCheck(t, "repository_id", fmt.Sprint(repositoryID)),
				Config:             config(""),
			},
			{
				Config:             config(""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config: config("data_wo_version = 1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_repository_action_secret.adopted", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("forgejo_repository_action_secret.adopted", "data_wo_version", "1"),
			},
		},
	})
}

func TestAccOrganizationActionSecretResourceWriteOnlyAdoption(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	testAccPreCheck(t)

	name := fmt.Sprintf("write-only-adoption-%d", time.Now().UnixNano())
	client := actionSecretAdoptionClient(t)
	_, _, err := client.CreateOrg(forgejo.CreateOrgOption{Name: name})
	require.NoError(t, err)
	t.Cleanup(func() {
		if _, err := client.DeleteOrg(name); err != nil {
			t.Errorf("delete adoption organization: %v", err)
		}
	})
	_, err = client.CreateOrgActionSecret(name, forgejo.CreateSecretOption{
		Name: "WRITE_ONLY_ADOPTED",
		Data: "existing-value",
	})
	require.NoError(t, err)

	config := func(version string) string {
		return providerConfig + fmt.Sprintf(`
resource "forgejo_organization_action_secret" "adopted" {
	organization    = %q
	name            = "WRITE_ONLY_ADOPTED"
	data_wo         = "adopted-value"
	%s
}`, name, version)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName:       "forgejo_organization_action_secret.adopted",
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateId:      name + "/WRITE_ONLY_ADOPTED",
				ImportStateCheck:   actionSecretAdoptionImportCheck(t, "organization", name),
				Config:             config(""),
			},
			{
				Config:             config(""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config: config("data_wo_version = 1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("forgejo_organization_action_secret.adopted", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("forgejo_organization_action_secret.adopted", "data_wo_version", "1"),
			},
		},
	})
}

func actionSecretAdoptionClient(t *testing.T) *forgejo.Client {
	t.Helper()
	client, err := forgejo.NewClient(
		forgejoTestHost,
		forgejo.SetToken(os.Getenv("FORGEJO_API_TOKEN")),
	)
	require.NoError(t, err)
	return client
}

func actionSecretAdoptionImportCheck(t *testing.T, parentAttribute, parentValue string) resource.ImportStateCheckFunc {
	t.Helper()
	return func(states []*terraform.InstanceState) error {
		if len(states) != 1 {
			return fmt.Errorf("expected one imported resource, got %d", len(states))
		}
		if got := states[0].Attributes[parentAttribute]; got != parentValue {
			return fmt.Errorf("expected %s %q, got %q", parentAttribute, parentValue, got)
		}
		if got := states[0].Attributes["name"]; got != "WRITE_ONLY_ADOPTED" {
			return fmt.Errorf("expected imported secret name, got %q", got)
		}
		return nil
	}
}
