# -------------------------------------------------------------------
# "Log in with Slack" for login.sequoia.garden
#
# Slack is a plain OpenID Connect provider ("Sign in with Slack"); the Slack
# app is only the client registration authentik uses to talk to it. The first
# Slack login of a workspace member creates their authentik account (thin
# provisioning) via the enrollment flow below; later logins go through the
# authentication flow. Existing hand-made accounts are linked by email.
#
# Accounts created this way land in the "Slack Community" group, distinct
# from "Members" (which gates paid-member applications and is managed by
# Multipass and the admin UI).
#
# Once logged in, users add a password, TOTP or WebAuthn in the stock user
# settings interface like any other user; nothing here restricts that.
#
# Objects: source + property mapping + group, enrollment flow (username
# prompt -> user write -> login), authentication flow (login), the
# identification stage that shows the Slack button, and the guard policies.
# -------------------------------------------------------------------

locals {
  slack_source_slug = "slack"
}

# --- Source ---------------------------------------------------------

resource "authentik_source_oauth" "slack" {
  # The name is the button label: "Continue with Sequoia Fabrica Slack".
  name          = "Sequoia Fabrica Slack"
  slug          = local.slack_source_slug
  provider_type = "openidconnect"

  # Promoted: rendered as a full-width primary button instead of a small
  # icon button. The brand CSS (brands.tf) then moves it above the
  # username/password form so Slack is the suggested way in and email login
  # is the fallback beneath.
  promoted = true

  consumer_key    = var.slack_client_id
  consumer_secret = var.slack_client_secret

  # Slack's OIDC endpoints. Callback registered in the Slack app:
  #   https://login.sequoia.garden/source/oauth/callback/slack/
  oidc_well_known_url = "https://slack.com/.well-known/openid-configuration"
  authorization_url   = "https://slack.com/openid/connect/authorize"
  access_token_url    = "https://slack.com/api/openid.connect.token"
  profile_url         = "https://slack.com/api/openid.connect.userInfo"
  oidc_jwks_url       = "https://slack.com/openid/connect/keys"

  # authentik requests `openid email profile` for the openidconnect type by
  # itself; those are also the only user-token scopes on the Slack app.
  additional_scopes = ""

  authentication_flow = authentik_flow.slack_authentication.uuid
  enrollment_flow     = var.slack_enrollment_enabled ? authentik_flow.slack_enrollment.uuid : null

  # Link to an existing account with the same email instead of creating a
  # duplicate. Slack only returns verified emails.
  user_matching_mode = "email_link"
  user_path_template = "goauthentik.io/sources/%(slug)s"

  property_mappings = [authentik_property_mapping_source_oauth.slack.id]
}

# Slack's userinfo has no `preferred_username`/`nickname`, so authentik's
# built-in mapping yields no username and the enrollment flow asks for one.
# This mapping supplies the rest and keeps the Slack identity on the user.
resource "authentik_property_mapping_source_oauth" "slack" {
  name       = "Slack: profile and workspace identity"
  expression = <<-EOT
    # `info` is the OpenID Connect userinfo response from Slack.
    picture = info.get("https://slack.com/user_image_192") or info.get("picture")
    return {
      "name": info.get("name") or "",
      "email": info.get("email"),
      "attributes": {
        # Used by AUTHENTIK_AVATARS=attributes.avatar,... (ansible env).
        "avatar": picture,
        "slack": {
          "user_id": info.get("https://slack.com/user_id") or info.get("sub"),
          "team_id": info.get("https://slack.com/team_id"),
          "team_domain": info.get("https://slack.com/team_domain"),
          "locale": info.get("locale"),
        },
      },
    }
  EOT
}

resource "authentik_group" "slack_community" {
  name = "Slack Community"
  attributes = jsonencode({
    description = "Accounts created or linked by logging in with Slack"
  })
}

# --- Guard policies -------------------------------------------------

# Both source flows may only be reached from a source login, never by
# visiting their URL directly (which would allow enrolling without Slack).
resource "authentik_policy_expression" "slack_if_sso" {
  name       = "slack-source-if-sso"
  expression = "return ak_is_sso_flow"
}

# Refuse identities from any Slack workspace but ours, at enrollment and at
# every later login. authentik's OAuth callback puts Slack's raw userinfo
# response into the flow context as `oauth_userinfo` before the flow's own
# policies are evaluated (authentik/sources/oauth/views/callback.py ->
# core/sources/flow_manager.py::_prepare_flow -> FlowPlanner.plan), so this
# reads Slack's team claim directly rather than anything we mapped. Missing
# claim => denied.
resource "authentik_policy_expression" "slack_team_gate" {
  name       = "slack-source-workspace-gate"
  expression = <<-EOT
    info = context.get("oauth_userinfo") or {}
    team_id = info.get("https://slack.com/team_id")
    if team_id != "${var.slack_team_id}":
        ak_message("This Slack account is not in the Sequoia Fabrica workspace.")
        ak_logger.warning("slack source: refused workspace", team_id=team_id, sub=info.get("sub"))
        return False
    return True
  EOT
}

# Ask for a username only when nothing provided one.
resource "authentik_policy_expression" "slack_if_no_username" {
  name       = "slack-source-enrollment-if-no-username"
  expression = "return 'username' not in context.get('prompt_data', {})"
}

# --- Enrollment flow: first Slack login creates the account -------------

resource "authentik_flow" "slack_enrollment" {
  name               = "Sequoia Fabrica Slack Enrollment"
  title              = "Welcome to the garden"
  slug               = "sequoia-fabrica-slack-enrollment"
  designation        = "enrollment"
  policy_engine_mode = "all"
  layout             = "stacked"
  denied_action      = "message"
  compatibility_mode = false
  background         = "sequoia_fabrica_flow_background.svg"
}

resource "authentik_policy_binding" "slack_enrollment_if_sso" {
  target = authentik_flow.slack_enrollment.uuid
  policy = authentik_policy_expression.slack_if_sso.id
  order  = 0
}

resource "authentik_policy_binding" "slack_enrollment_team_gate" {
  target = authentik_flow.slack_enrollment.uuid
  policy = authentik_policy_expression.slack_team_gate.id
  order  = 10
}

resource "authentik_stage_prompt_field" "slack_username" {
  name      = "slack-source-enrollment-field-username"
  field_key = "username"
  label     = "Username"
  type      = "username"
  required  = true
  order     = 100
  sub_text  = "How you appear in the members directory. Letters, digits, dots, dashes and underscores."

  # Suggest the local part of the Slack email; the user can change it.
  initial_value_expression = true
  initial_value            = <<-EOT
    return (prompt_context.get("email") or "").split("@")[0].lower()
  EOT
}

resource "authentik_stage_prompt" "slack_enrollment" {
  name   = "slack-source-enrollment-prompt"
  fields = [authentik_stage_prompt_field.slack_username.id]
}

resource "authentik_stage_user_write" "slack_enrollment" {
  name               = "slack-source-enrollment-write"
  user_creation_mode = "always_create"
  create_users_group = authentik_group.slack_community.id
  user_type          = "internal"
}

resource "authentik_flow_stage_binding" "slack_enrollment_prompt" {
  target               = authentik_flow.slack_enrollment.uuid
  stage                = authentik_stage_prompt.slack_enrollment.id
  order                = 10
  re_evaluate_policies = true
}

resource "authentik_policy_binding" "slack_enrollment_prompt_if_no_username" {
  target = authentik_flow_stage_binding.slack_enrollment_prompt.id
  policy = authentik_policy_expression.slack_if_no_username.id
  order  = 0
}

resource "authentik_flow_stage_binding" "slack_enrollment_write" {
  target = authentik_flow.slack_enrollment.uuid
  stage  = authentik_stage_user_write.slack_enrollment.id
  order  = 20
}

resource "authentik_flow_stage_binding" "slack_enrollment_login" {
  target = authentik_flow.slack_enrollment.uuid
  stage  = data.authentik_stage.default_source_enrollment_login.id
  order  = 30
}

# --- Authentication flow: returning Slack users -------------------------

resource "authentik_flow" "slack_authentication" {
  name               = "Sequoia Fabrica Slack Authentication"
  title              = "Welcome back to the garden"
  slug               = "sequoia-fabrica-slack-authentication"
  designation        = "authentication"
  policy_engine_mode = "all"
  layout             = "stacked"
  denied_action      = "message"
  compatibility_mode = false
  background         = "sequoia_fabrica_flow_background.svg"
}

resource "authentik_policy_binding" "slack_authentication_if_sso" {
  target = authentik_flow.slack_authentication.uuid
  policy = authentik_policy_expression.slack_if_sso.id
  order  = 0
}

resource "authentik_policy_binding" "slack_authentication_team_gate" {
  target = authentik_flow.slack_authentication.uuid
  policy = authentik_policy_expression.slack_team_gate.id
  order  = 10
}

resource "authentik_flow_stage_binding" "slack_authentication_login" {
  target = authentik_flow.slack_authentication.uuid
  stage  = data.authentik_stage.default_source_authentication_login.id
  order  = 10
}

# --- Identification stage with the Slack button -------------------------
#
# Same shape as default-authentication-identification (username or email,
# password inline) plus `sources`. setup-env.sh prints the live default
# stage's recovery/enrollment/passwordless/captcha settings so any of those
# that are configured there can be mirrored here before applying.

resource "authentik_stage_identification" "sequoia_fabrica" {
  name                      = "sequoia-fabrica-authentication-identification"
  user_fields               = ["username", "email"]
  password_stage            = data.authentik_stage.default_authentication_password.id
  case_insensitive_matching = true
  show_matched_user         = true
  show_source_labels        = true
  sources                   = [authentik_source_oauth.slack.uuid]
}
