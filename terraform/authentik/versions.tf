terraform {
  # >= 1.7 so `import` blocks may take their `id` from a variable (imports.tf).
  required_version = ">= 1.7"

  required_providers {
    authentik = {
      source  = "goauthentik/authentik"
      version = "~> 2026.5"
    }
  }
}

# Configured via environment variables (see setup-env.sh):
#   AUTHENTIK_URL   = "https://login.sequoia.garden/"
#   AUTHENTIK_TOKEN = "<ephemeral api token>"
provider "authentik" {}
