# -------------------------------------------------------------------
# Read-only references to objects created by authentik's own blueprints.
#
# The branded authentication flow (flows.tf) binds the *default* stages so the
# behaviour (identification with inline password, MFA validation, login, any
# configured OAuth sources) stays exactly what the instance has today; only
# the presentation changes.
# -------------------------------------------------------------------

data "authentik_flow" "default_invalidation" {
  slug = "default-invalidation-flow"
}

data "authentik_flow" "default_user_settings" {
  slug = "default-user-settings-flow"
}

data "authentik_stage" "default_authentication_identification" {
  name = "default-authentication-identification"
}

data "authentik_stage" "default_authentication_mfa_validation" {
  name = "default-authentication-mfa-validation"
}

data "authentik_stage" "default_authentication_login" {
  name = "default-authentication-login"
}
