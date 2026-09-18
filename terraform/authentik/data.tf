# -------------------------------------------------------------------
# Read-only references to objects created by authentik's own blueprints or
# managed outside this workspace (groups, users). Nothing here is changed
# by Terraform; it is only looked up so resources can point at it.
# -------------------------------------------------------------------

# --- Flows ---

data "authentik_flow" "default_authentication" {
  slug = "default-authentication-flow"
}

data "authentik_flow" "default_invalidation" {
  slug = "default-invalidation-flow"
}

data "authentik_flow" "default_user_settings" {
  slug = "default-user-settings-flow"
}

data "authentik_flow" "default_implicit_consent" {
  slug = "default-provider-authorization-implicit-consent"
}

data "authentik_flow" "default_explicit_consent" {
  slug = "default-provider-authorization-explicit-consent"
}

data "authentik_flow" "default_provider_invalidation" {
  slug = "default-provider-invalidation-flow"
}

# --- Stages bound into the branded authentication flow (flows.tf) ---

data "authentik_stage" "default_authentication_identification" {
  name = "default-authentication-identification"
}

data "authentik_stage" "default_authentication_mfa_validation" {
  name = "default-authentication-mfa-validation"
}

data "authentik_stage" "default_authentication_login" {
  name = "default-authentication-login"
}

# --- Stages reused by the Slack source flows (sources.tf) ---

data "authentik_stage" "default_authentication_password" {
  name = "default-authentication-password"
}

data "authentik_stage" "default_source_enrollment_login" {
  name = "default-source-enrollment-login"
}

data "authentik_stage" "default_source_authentication_login" {
  name = "default-source-authentication-login"
}

# --- Default scope mappings (OAuth2 providers; proxy providers get theirs
#     from authentik automatically, see providers.tf) ---

data "authentik_property_mapping_provider_scope" "openid" {
  managed = "goauthentik.io/providers/oauth2/scope-openid"
}

data "authentik_property_mapping_provider_scope" "email" {
  managed = "goauthentik.io/providers/oauth2/scope-email"
}

data "authentik_property_mapping_provider_scope" "profile" {
  managed = "goauthentik.io/providers/oauth2/scope-profile"
}

# --- Signing certificate ---

data "authentik_certificate_key_pair" "self_signed" {
  name = "authentik Self-signed Certificate"
}

# --- Groups and users referenced by application access bindings ---
# Group membership is managed by hand (and by Multipass), not here.

data "authentik_group" "authentik_admins" {
  name = "authentik Admins"
}

data "authentik_group" "members" {
  name = "Members"
}

data "authentik_group" "security_system_operators" {
  name = "Security-System-Operators"
}

data "authentik_user" "jof" {
  username = "jof"
}
