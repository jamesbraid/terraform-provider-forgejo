package provider_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3/models"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
)

func TestOAuth2AuthSourceImportDoesNotReplaceOmittedFields(t *testing.T) {
	var updates []models.OAuth2AuthSourceOptions
	deletes := 0
	unexpectedMutations := 0
	sourceJSON := `{
		"id":42,"name":"Rauthy","is_active":true,"provider":"openidConnect",
		"client_id":"forgejo","openid_connect_auto_discovery_url":"https://auth.example/.well-known/openid-configuration",
		"scopes":["openid","profile"],"icon_url":"https://auth.example/icon.svg",
		"custom_url_mapping":{"auth_url":"https://auth.example/authorize","token_url":"https://auth.example/token","profile_url":"","email_url":"","tenant":""},
		"attribute_ssh_public_key":"ssh","required_claim_name":"role","required_claim_value":"member",
		"group_claim_name":"groups","admin_group":"admins","group_team_map":"{}","group_team_map_removal":true,
		"dyn_group_maps":"{}","dyn_group_maps_removal":true,"quota_group_claim_name":"quota","quota_group_map":"{}",
		"quota_group_map_removal":true,"restricted_group":"guests","skip_local_two_fa":true,"allow_username_change":true
	}`
	var current models.OAuth2AuthSource
	require.NoError(t, json.Unmarshal([]byte(sourceJSON), &current))
	original := current
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete && r.URL.Path == "/api/v1/admin/auths/oauth2/42" {
			deletes++
			w.WriteHeader(http.StatusNoContent) // Test harness destroys imported resources during cleanup.
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/api/v1/admin/auths/oauth2/42" {
			var updated models.OAuth2AuthSourceOptions
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			updates = append(updates, updated)
			current = updated.Source
			_ = json.NewEncoder(w).Encode(current)
			return
		}
		if r.Method != http.MethodGet {
			unexpectedMutations++
			http.Error(w, "unexpected mutation", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"16.0.3"}`))
		case "/api/v1/admin/auths/oauth2/42":
			_ = json.NewEncoder(w).Encode(current)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	config := func(secret, version, fields string) string {
		return fmt.Sprintf(`
provider "forgejo" {
  host      = %q
  api_token = "test-token"
}
resource "forgejo_oauth2_auth_source" "rauthy" {
  name = "Rauthy"
  oauth2_provider = "openidConnect"
  client_id = "forgejo"
  client_secret_wo = %q
  %s
  %s
}
`, server.URL, secret, version, fields)
	}
	initial := config("not-read-by-forgejo", "", "")
	rotated := config("rotated-client-secret", "client_secret_wo_version = 1", "")
	cleared := config("rotated-client-secret", "client_secret_wo_version = 1", `
  is_active = false
  scopes = []
  icon_url = ""
  custom_url_mapping = { auth_url = "" }
`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{ResourceName: "forgejo_oauth2_auth_source.rauthy", Config: initial,
				ImportState: true, ImportStatePersist: true, ImportStateId: "42"},
			{Config: initial, PlanOnly: true, ExpectNonEmptyPlan: false},
			{Config: rotated},
			{Config: rotated, PlanOnly: true, ExpectNonEmptyPlan: false},
			{Config: cleared},
			{Config: cleared, PlanOnly: true, ExpectNonEmptyPlan: false},
		},
	})
	require.Len(t, updates, 2, "only the revision and explicit field changes should issue updates")
	require.Equal(t, 1, deletes, "the test harness destroys the imported mock resource at teardown")
	require.Zero(t, unexpectedMutations)
	require.Equal(t, "rotated-client-secret", updates[0].ClientSecret)
	require.Equal(t, original, updates[0].Source, "revision update must preserve omitted imported fields")
	require.Empty(t, updates[1].ClientSecret, "non-secret update must preserve the existing secret")
	expected := original
	expected.IsActive = false
	expected.Scopes = []string{}
	expected.IconURL = ""
	mapping := *original.CustomURLMapping
	mapping.AuthURL = ""
	expected.CustomURLMapping = &mapping
	require.Equal(t, expected, updates[1].Source, "explicit zero values must clear without resetting omitted fields")
}
