# -------------------------------------------------------------------
# Branded authentication flow for login.sequoia.garden
#
# Same shape as authentik's default-authentication-flow (identification with
# the password field inline, MFA validation, user login) but owned by us so
# the title and layout carry the Sequoia Fabrica voice. The stages are the
# blueprint-managed defaults (data.tf), so OAuth sources or MFA settings that
# are already configured on them keep working here unchanged.
#
# The stock default-authentication-flow is left in place as a fallback: point
# the brand back at it (brands.tf) to roll back without deleting anything.
# -------------------------------------------------------------------

resource "authentik_flow" "sequoia_fabrica_authentication" {
  name               = "Sequoia Fabrica Authentication"
  title              = "Welcome back to the garden"
  slug               = "sequoia-fabrica-authentication"
  designation        = "authentication"
  policy_engine_mode = "any"
  layout             = "stacked"
  denied_action      = "message_continue"
  compatibility_mode = false

  # The provider fills an unset `background` with authentik's stock JPEG
  # (/static/dist/assets/images/flow_background.jpg), which would override the
  # brand-level default, so pin the same brand asset here explicitly.
  background = "sequoia_fabrica_flow_background.svg"
}

resource "authentik_flow_stage_binding" "sequoia_fabrica_auth_identification" {
  target = authentik_flow.sequoia_fabrica_authentication.uuid
  stage  = data.authentik_stage.default_authentication_identification.id
  order  = 10
}

resource "authentik_flow_stage_binding" "sequoia_fabrica_auth_mfa_validation" {
  target = authentik_flow.sequoia_fabrica_authentication.uuid
  stage  = data.authentik_stage.default_authentication_mfa_validation.id
  order  = 30
}

resource "authentik_flow_stage_binding" "sequoia_fabrica_auth_login" {
  target = authentik_flow.sequoia_fabrica_authentication.uuid
  stage  = data.authentik_stage.default_authentication_login.id
  order  = 100
}
