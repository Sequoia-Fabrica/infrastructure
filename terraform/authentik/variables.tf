# No secrets live here. Everything that is instance-specific (object UUIDs to
# import) is discovered from the API by setup-env.sh and written to the
# gitignored imports.generated.tf. The Slack app credentials are read by
# setup-env.sh from the ansible vault (group_vars/all.yml, key
# authentik.sources.slack) and exported as TF_VAR_slack_* for the session.

variable "authentik_domain" {
  description = "Domain the brand matches on (also the public host of the instance)"
  type        = string
  default     = "login.sequoia.garden"
}

# --- Slack "Sign in with Slack" source (sources.tf) ---

variable "slack_client_id" {
  description = "Client ID of the Slack app (Basic Information -> App Credentials)"
  type        = string
}

variable "slack_client_secret" {
  description = "Client Secret of the Slack app; comes from the ansible vault, never from a file in git"
  type        = string
  sensitive   = true
}

variable "slack_team_id" {
  description = <<-EOT
    Slack workspace (team) ID, e.g. T0123ABCD. Required: Slack identities from
    any other workspace are refused at enrollment and at every login, so the
    source stays safe even if the Slack app is ever distributed.
  EOT
  type        = string

  validation {
    condition     = can(regex("^T[A-Z0-9]{8,}$", var.slack_team_id))
    error_message = "slack_team_id must be a Slack team ID like T0123ABCD (set authentik.sources.slack.team_id in the ansible vault group_vars)."
  }
}

variable "slack_enrollment_enabled" {
  description = <<-EOT
    true: the first Slack login creates an authentik account (thin
    provisioning). false: only users already linked to Slack, or matched by
    email, can log in with Slack; unknown identities are turned away.
  EOT
  type        = bool
  default     = true
}
