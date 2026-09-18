# -------------------------------------------------------------------
# Applications (what members see in the library at login.sequoia.garden)
#
# meta_icon values are bare media keys under application-icons/, uploaded
# through the admin UI. Access restrictions live in policies.tf.
# -------------------------------------------------------------------

resource "authentik_application" "adminer" {
  name               = "Adminer"
  slug               = "adminer"
  protocol_provider  = authentik_provider_proxy.adminer.id
  meta_launch_url    = "https://adminer.sequoia.garden"
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "chat" {
  name               = "Chat"
  slug               = "chat"
  protocol_provider  = authentik_provider_oauth2.chat.id
  meta_launch_url    = "https://chat.sequoia.garden/"
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "etherpad" {
  name               = "Etherpad"
  slug               = "etherpad"
  protocol_provider  = authentik_provider_proxy.etherpad.id
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "frigate" {
  name               = "Frigate"
  slug               = "frigate"
  protocol_provider  = authentik_provider_proxy.frigate.id
  meta_icon          = "application-icons/frigate_rK1YXBz.png"
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "grafana" {
  name               = "Grafana"
  slug               = "grafana"
  protocol_provider  = authentik_provider_oauth2.grafana.id
  meta_launch_url    = "https://grafana.sequoia.garden/login/generic_oauth"
  meta_icon          = "application-icons/54ac468e-grafana_logo_swirl-events.webp"
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "immich" {
  name               = "Immich"
  slug               = "immich"
  protocol_provider  = authentik_provider_oauth2.immich.id
  meta_launch_url    = "https://photos.sequoia.garden/auth/login?autoLaunch=0"
  meta_icon          = "application-icons/immich-logo.png"
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "multipass" {
  name               = "Multipass"
  slug               = "multipass"
  protocol_provider  = authentik_provider_proxy.multipass.id
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "multipass_test" {
  name               = "multipass-test"
  slug               = "multipass-test"
  protocol_provider  = authentik_provider_proxy.multipass_test.id
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "uptime_kuma" {
  name               = "Uptime Kuma"
  slug               = "uptime-kuma"
  protocol_provider  = authentik_provider_proxy.uptime_kuma.id
  open_in_new_tab    = false
  policy_engine_mode = "any"
}

resource "authentik_application" "utilities" {
  name               = "utilities"
  slug               = "utilities"
  protocol_provider  = authentik_provider_proxy.utilities.id
  open_in_new_tab    = false
  policy_engine_mode = "any"
}
