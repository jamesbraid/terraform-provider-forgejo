resource "forgejo_oauth2_auth_source" "example" {
  name                              = "Company SSO"
  oauth2_provider                   = "openidConnect"
  client_id                         = "forgejo"
  client_secret_wo                  = var.oauth2_client_secret
  client_secret_wo_version          = 1
  openid_connect_auto_discovery_url = "https://sso.example.com/.well-known/openid-configuration"
  scopes                            = ["openid", "profile", "email"]
  is_active                         = true
}
