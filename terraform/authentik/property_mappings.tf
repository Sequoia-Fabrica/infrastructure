# -------------------------------------------------------------------
# Custom scope mappings
# -------------------------------------------------------------------

# Immich has one shared account for the whole space; this scope makes every
# member present as that account. Also attached to the Etherpad and Uptime
# Kuma proxy providers, where it is harmless (proxy auth ignores the claims).
resource "authentik_property_mapping_provider_scope" "immich_same_users" {
  name        = "immich_same_users"
  scope_name  = "immich_same_users"
  description = "All Sequoia Fabrica Members access the same user in Immich"
  expression  = <<-EOT
    return {
      "sub": "Sequoia-Fabrica",
      "preferred_username": "Sequoia-Fabrica",
      "email": "seqfab.infra@gmail.com"
    }
  EOT
}
