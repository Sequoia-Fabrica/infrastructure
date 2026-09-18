# -------------------------------------------------------------------
# Application access: who may see and launch each application.
#
# These are direct group/user bindings on the application (no expression
# policy in between). Applications without a binding are open to every
# authenticated user.
# -------------------------------------------------------------------

resource "authentik_policy_binding" "adminer_authentik_admins" {
  target  = authentik_application.adminer.uuid
  group   = data.authentik_group.authentik_admins.id
  order   = 0
  timeout = 30
}

resource "authentik_policy_binding" "frigate_security_system_operators" {
  target  = authentik_application.frigate.uuid
  group   = data.authentik_group.security_system_operators.id
  order   = 0
  timeout = 30
}

resource "authentik_policy_binding" "utilities_members" {
  target  = authentik_application.utilities.uuid
  group   = data.authentik_group.members.id
  order   = 0
  timeout = 30
}

# Staging instance: only its developer can see it.
resource "authentik_policy_binding" "multipass_test_jof" {
  target  = authentik_application.multipass_test.uuid
  user    = data.authentik_user.jof.pk
  order   = 0
  timeout = 30
}
