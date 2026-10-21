data "hivemind_forager_integration_teams" "available" {}

output "integration_teams" {
  value = data.hivemind_forager_integration_teams.available.teams
}
