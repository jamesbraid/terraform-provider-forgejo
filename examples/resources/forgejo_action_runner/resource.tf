resource "forgejo_action_runner" "example" {
  scope       = "organization"
  owner       = "example"
  name        = "example-runner"
  description = "Runner managed by Terraform"
}
