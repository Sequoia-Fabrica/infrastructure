# -------------------------------------------------------------------
# Providers: OAuth2/OIDC for apps that speak OIDC themselves, proxy
# providers (served by authentik's embedded outpost) for the rest.
#
# Client secrets are NOT declared: the attribute is optional+computed, so
# the imported state carries the live secret and nothing sensitive lives in
# git or in TF_VAR_* variables. Rotate a secret in the admin UI, then plan.
#
# Values mirror the live instance exactly (import-only first plan). Several
# providers pin `authentication_flow` to the stock default flow, which means
# those app logins bypass the branded flow in flows.tf; that is called out
# per provider and is a deliberate follow-up, not changed here.
# -------------------------------------------------------------------

locals {
  # Grant types authentik enables by default on providers created in the UI.
  oauth2_default_grant_types = [
    "authorization_code",
    "hybrid",
    "implicit",
    "client_credentials",
    "password",
    "urn:ietf:params:oauth:grant-type:device_code",
    "refresh_token",
  ]

  oidc_scopes = [
    data.authentik_property_mapping_provider_scope.openid.id,
    data.authentik_property_mapping_provider_scope.email.id,
    data.authentik_property_mapping_provider_scope.profile.id,
  ]

  # Services proxied by the embedded outpost live on nursery.
  nursery_internal = "http://nursery.xylem.sequoiafabrica.org"
}

# --- OAuth2 / OIDC -------------------------------------------------

resource "authentik_provider_oauth2" "grafana" {
  name      = "Grafana"
  client_id = "m4mO9fx6Yh6iVKxoJBM7wNdrgoblHI2lxWfsvIJr"

  # Pinned to the stock flow (predates the branded flow).
  authentication_flow = data.authentik_flow.default_authentication.id
  authorization_flow  = data.authentik_flow.default_implicit_consent.id
  invalidation_flow   = data.authentik_flow.default_provider_invalidation.id
  signing_key         = data.authentik_certificate_key_pair.self_signed.id

  client_type = "confidential"
  grant_types = local.oauth2_default_grant_types

  allowed_redirect_uris = [
    {
      matching_mode     = "strict"
      url               = "https://grafana.sequoia.garden/login/generic_oauth"
      redirect_uri_type = "authorization"
    },
  ]

  access_code_validity       = "minutes=1"
  access_token_validity      = "minutes=5"
  refresh_token_validity     = "days=30"
  refresh_token_threshold    = "seconds=0"
  include_claims_in_id_token = true
  logout_method              = "backchannel"
  sub_mode                   = "user_email"
  issuer_mode                = "per_provider"

  property_mappings = local.oidc_scopes
}

resource "authentik_provider_oauth2" "chat" {
  name      = "Provider for Chat"
  client_id = "5WXNpPqj2OdtqXjki5x2JuhgvDxIXrY2X2P2BHC8"

  # Pinned to the stock flow (predates the branded flow).
  authentication_flow = data.authentik_flow.default_authentication.id
  authorization_flow  = data.authentik_flow.default_implicit_consent.id
  invalidation_flow   = data.authentik_flow.default_provider_invalidation.id
  signing_key         = data.authentik_certificate_key_pair.self_signed.id

  client_type = "confidential"
  grant_types = local.oauth2_default_grant_types

  allowed_redirect_uris = [
    {
      matching_mode     = "strict"
      url               = "https://chat.sequoia.garden/oauth/oidc/callback"
      redirect_uri_type = "authorization"
    },
  ]

  access_code_validity       = "minutes=1"
  access_token_validity      = "minutes=5"
  refresh_token_validity     = "days=30"
  refresh_token_threshold    = "seconds=0"
  include_claims_in_id_token = true
  logout_method              = "backchannel"
  sub_mode                   = "user_email"
  issuer_mode                = "global"

  property_mappings = local.oidc_scopes
}

resource "authentik_provider_oauth2" "immich" {
  name      = "Provider for Immich"
  client_id = "RaNDw7jmltr5JDx3DlDUtJEcYynmTV4Mw3n5BzUo"

  # Pinned to the stock flow (predates the branded flow).
  authentication_flow = data.authentik_flow.default_authentication.id
  authorization_flow  = data.authentik_flow.default_implicit_consent.id
  invalidation_flow   = data.authentik_flow.default_provider_invalidation.id
  signing_key         = data.authentik_certificate_key_pair.self_signed.id

  client_type = "confidential"
  grant_types = local.oauth2_default_grant_types

  allowed_redirect_uris = [
    {
      matching_mode     = "strict"
      url               = "https://photos.sequoia.garden/auth/login"
      redirect_uri_type = "authorization"
    },
    {
      matching_mode     = "strict"
      url               = "https://photos.sequoia.garden/user-settings"
      redirect_uri_type = "authorization"
    },
    {
      # Immich mobile app
      matching_mode     = "strict"
      url               = "app.immich:///oauth-callback"
      redirect_uri_type = "authorization"
    },
  ]

  access_code_validity       = "minutes=1"
  access_token_validity      = "minutes=5"
  refresh_token_validity     = "days=30"
  refresh_token_threshold    = "seconds=0"
  include_claims_in_id_token = true
  logout_method              = "backchannel"
  sub_mode                   = "hashed_user_id"
  issuer_mode                = "per_provider"

  property_mappings = concat(
    [authentik_property_mapping_provider_scope.immich_same_users.id],
    local.oidc_scopes,
  )
}

# --- Proxy (embedded outpost) ----------------------------------------
# The embedded outpost's provider list is maintained by authentik itself
# when a proxy provider is created; it is not managed here.
#
# property_mappings is deliberately NOT declared on proxy providers. The
# Terraform provider only reads that field back when the state already has
# a value, so on our stateless import-every-session model it would show as
# a perpetual "+ property_mappings" change. authentik applies the standard
# proxy scope set (ak_proxy, openid, email, profile, entitlements) to every
# proxy provider on its own, and an update request without the field leaves
# the live mappings untouched. Etherpad and Uptime Kuma additionally carry
# the vestigial immich_same_users scope from their creation; it is unused
# by proxy auth and is left as-is.

resource "authentik_provider_proxy" "adminer" {
  name          = "Provider for Adminer"
  mode          = "proxy"
  external_host = "https://adminer.sequoia.garden"
  internal_host = "${local.nursery_internal}:3006"

  # Pinned to the stock flow (predates the branded flow).
  authentication_flow = data.authentik_flow.default_authentication.id
  authorization_flow  = data.authentik_flow.default_explicit_consent.id
  invalidation_flow   = data.authentik_flow.default_provider_invalidation.id

  internal_host_ssl_validation = false
  intercept_header_auth        = true
  access_token_validity        = "hours=0"
  refresh_token_validity       = "days=30"
}

resource "authentik_provider_proxy" "etherpad" {
  name          = "Provider for Etherpad"
  mode          = "proxy"
  external_host = "https://etherpad.sequoia.garden"
  internal_host = "${local.nursery_internal}:9001"

  authorization_flow = data.authentik_flow.default_explicit_consent.id
  invalidation_flow  = data.authentik_flow.default_provider_invalidation.id

  internal_host_ssl_validation = true
  intercept_header_auth        = true
  access_token_validity        = "hours=24"
  refresh_token_validity       = "days=30"
}

resource "authentik_provider_proxy" "frigate" {
  name          = "Provider for Frigate"
  mode          = "proxy"
  external_host = "https://frigate.sequoia.garden"
  internal_host = "${local.nursery_internal}:5000"

  # Pinned to the stock flow (predates the branded flow).
  authentication_flow = data.authentik_flow.default_authentication.id
  authorization_flow  = data.authentik_flow.default_implicit_consent.id
  invalidation_flow   = data.authentik_flow.default_provider_invalidation.id

  internal_host_ssl_validation = false
  intercept_header_auth        = true
  access_token_validity        = "hours=24"
  refresh_token_validity       = "days=30"
}

resource "authentik_provider_proxy" "multipass" {
  name          = "Provider for Multipass"
  mode          = "proxy"
  external_host = "https://multipass.sequoia.garden/"
  internal_host = "${local.nursery_internal}:3005/"

  authorization_flow = data.authentik_flow.default_implicit_consent.id
  invalidation_flow  = data.authentik_flow.default_provider_invalidation.id

  # Public card page and static assets need no login.
  skip_path_regex = "/public/card\n/static/css/.*\n/static/js/.*"

  internal_host_ssl_validation = false
  intercept_header_auth        = true
  access_token_validity        = "hours=24"
  refresh_token_validity       = "days=30"
}

resource "authentik_provider_proxy" "multipass_test" {
  name          = "Provider for multipass-test"
  mode          = "proxy"
  external_host = "https://multipass-test.sequoia.garden/"
  internal_host = "http://ash.cloudforest-perch.ts.net:3000/"

  authorization_flow = data.authentik_flow.default_implicit_consent.id
  invalidation_flow  = data.authentik_flow.default_provider_invalidation.id

  skip_path_regex = "/public/card\n/static/css/.*\n/static/js/.*"

  internal_host_ssl_validation = false
  intercept_header_auth        = true
  access_token_validity        = "hours=24"
  refresh_token_validity       = "days=30"
}

resource "authentik_provider_proxy" "uptime_kuma" {
  name          = "Provider for Uptime Kuma"
  mode          = "proxy"
  external_host = "https://uptime-kuma.sequoia.garden"
  internal_host = "${local.nursery_internal}:3002"

  authorization_flow = data.authentik_flow.default_implicit_consent.id
  invalidation_flow  = data.authentik_flow.default_provider_invalidation.id

  internal_host_ssl_validation = true
  intercept_header_auth        = true
  access_token_validity        = "hours=24"
  refresh_token_validity       = "days=30"
}

resource "authentik_provider_proxy" "utilities" {
  name          = "Provider for utilities"
  mode          = "proxy"
  external_host = "https://utilities.sequoia.garden"
  internal_host = "${local.nursery_internal}:8000"

  # Pinned to the stock flow (predates the branded flow).
  authentication_flow = data.authentik_flow.default_authentication.id
  authorization_flow  = data.authentik_flow.default_explicit_consent.id
  invalidation_flow   = data.authentik_flow.default_provider_invalidation.id

  # Sign-in endpoints and static assets are handled by the app itself.
  skip_path_regex = "/api/submit_signin\n/signin\n/static/.*"

  internal_host_ssl_validation = false
  intercept_header_auth        = true
  access_token_validity        = "hours=24"
  refresh_token_validity       = "days=30"
}
