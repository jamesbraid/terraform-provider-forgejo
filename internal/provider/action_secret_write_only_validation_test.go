package provider_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestActionSecretWriteOnlyValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/version" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"9.0.0"}`))
	}))
	t.Cleanup(server.Close)

	tests := map[string]struct {
		resourceType string
		parent       string
	}{
		"organization": {
			resourceType: "forgejo_organization_action_secret",
			parent:       `organization = "example"`,
		},
		"repository": {
			resourceType: "forgejo_repository_action_secret",
			parent:       "repository_id = 1",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:             actionSecretValidationConfig(server.URL, test.resourceType, test.parent, `data_wo = "existing-value"`),
						PlanOnly:           true,
						ExpectNonEmptyPlan: true,
					},
					{
						Config: actionSecretValidationConfig(server.URL, test.resourceType, test.parent, `
	data_wo         = "existing-value"
	data_wo_version = null`),
						PlanOnly:           true,
						ExpectNonEmptyPlan: true,
					},
					{
						Config:      actionSecretValidationConfig(server.URL, test.resourceType, test.parent, "data_wo_version = 1"),
						PlanOnly:    true,
						ExpectError: regexp.MustCompile(`Attribute "data_wo" must be specified`),
					},
				},
			})
		})
	}
}

func actionSecretValidationConfig(host, resourceType, parent, data string) string {
	return fmt.Sprintf(`
provider "forgejo" {
	host      = %q
	api_token = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}

resource %q "validation" {
	%s
	name = "VALIDATION_SECRET"
	%s
}
`, host, resourceType, parent, data)
}
