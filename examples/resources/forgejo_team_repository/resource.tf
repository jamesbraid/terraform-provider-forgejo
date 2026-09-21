terraform {
  required_providers {
    forgejo = {
      source = "svalabs/forgejo"
    }
  }
}

provider "forgejo" {
  host = "http://localhost:3000"
}

resource "forgejo_organization" "example" {
  name = "example"
}

resource "forgejo_repository" "example" {
  owner = forgejo_organization.example.name
  name  = "example"
}

resource "forgejo_team" "example" {
  organization_id           = forgejo_organization.example.id
  name                      = "example"
  includes_all_repositories = false

  units_map = {
    "repo.code" = "read"
  }
}

resource "forgejo_team_repository" "example" {
  team_id       = forgejo_team.example.id
  repository_id = forgejo_repository.example.id
}

# Import using the numeric team and repository IDs.
import {
  id = "1/42"
  to = forgejo_team_repository.example
}
