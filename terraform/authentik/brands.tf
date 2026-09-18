# -------------------------------------------------------------------
# Brand: Sequoia Fabrica styling for login.sequoia.garden
#
# Media keys (branding_logo, branding_favicon, branding_default_flow_background)
# are bare filenames under authentik's "public/" media prefix. The files are
# placed on the host by ansible (roles/sequoia_fabrica/tasks/authentik.yml)
# into /opt/authentik/data/media/public/, and authentik serves them through
# signed /files/media/public/... URLs. Run `make ansible` before applying a
# change that adds a new asset.
#
# Palette is the landing page's (documentation/brand-style-guide.md):
#   emerald-800 #065f46  primary / nav & footer
#   emerald-900 #064e3b  deep background
#   tea green   #c3f9c3  wordmark on emerald, dark-mode ink
#   orange-700  #c2410c  links (landing page body links)
#   white smoke #f7f4f3  light card / page background
# -------------------------------------------------------------------

resource "authentik_brand" "sequoia_fabrica" {
  domain = var.authentik_domain
  # Matched by domain; the instance does not mark this brand as default and
  # we keep it that way to leave the stock authentik-default brand alone.
  default          = false
  branding_title   = "Sequoia Fabrica"
  branding_logo    = "sequoia_fabrica_lockup.svg"
  branding_favicon = "sequoia_fabrica_favicon.png"

  # Brand-wide default background for every flow (login, recovery, consent,
  # user settings...) that has no background of its own. Flows managed here
  # also set it explicitly, see flows.tf for why.
  branding_default_flow_background = "sequoia_fabrica_flow_background.svg"

  flow_authentication = authentik_flow.sequoia_fabrica_authentication.uuid
  flow_invalidation   = data.authentik_flow.default_invalidation.id
  flow_user_settings  = data.authentik_flow.default_user_settings.id

  # Footer links are NOT a brand property in this authentik version: they
  # live in the global system settings (authentik_system_settings.footer_links,
  # Admin UI -> System -> Settings). That resource is a singleton whose other
  # fields we have not adopted, so footer links stay managed by hand for now.

  # chomp() drops the heredoc's trailing newline so this matches exactly what
  # authentik stores; otherwise the one-byte difference shows as perpetual
  # plan drift on every run.
  branding_custom_css = chomp(<<-CSS
    /*
     * Sequoia Fabrica brand -- login.sequoia.garden
     * Managed by terraform/authentik/brands.tf. Colours from
     * documentation/brand-style-guide.md.
     *
     * CSS custom properties cascade through shadow DOM boundaries, so
     * overriding the PatternFly / authentik variables here themes the
     * login flow, the user library and the admin UI alike.
     */

    :root {
      /* Emerald primary: buttons, active nav, focus rings */
      --ak-accent: #065f46;
      --pf-global--primary-color--100: #065f46;
      --pf-global--primary-color--200: #047857;
      --pf-global--primary-color--light-100: #10b981;
      --pf-global--active-color--100: #065f46;
      --pf-global--active-color--300: #10b981;

      /* Landing-page orange links */
      --pf-global--link--Color: #c2410c;
      --pf-global--link--Color--hover: #7c2d12;
      --pf-global--link--Color--light: #fb923c;
      --pf-global--link--Color--light--hover: #fdba74;
      --pf-global--link--Color--dark: #fb923c;
      --pf-global--link--Color--dark--hover: #fdba74;

      /* Light theme: soft white-smoke surfaces */
      --pf-global--BackgroundColor--100: #ffffff;
      --pf-global--BackgroundColor--200: #f7f4f3;
      --pf-global--BackgroundColor--light-300: #f7f4f3;

      /* Dark theme (authentik's own tokens): deep emerald instead of grey */
      --ak-dark-background: #022c22;
      --ak-dark-background-darker: #011a14;
      --ak-dark-background-light: #05231c;
      --ak-dark-background-light-ish: #073a2e;
      --ak-dark-background-lighter: #0a4a3b;
      --ak-dark-foreground: #f3fef3;
      --ak-dark-foreground-darker: #c3f9c3;

      /* Admin / user interface chrome */
      --pf-c-page__header--BackgroundColor: #022c22;
      --pf-c-page__sidebar--BackgroundColor: #064e3b;
      --pf-c-nav__link--m-current--after--BorderColor: #c3f9c3;
      --pf-c-nav__link--hover--after--BorderColor: #c3f9c3;

      /* System font stack, matching the landing page (Tailwind default) */
      --pf-global--FontFamily--sans-serif:
        ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto,
        "Helvetica Neue", Arial, "Noto Sans", sans-serif;
    }

    /* Login card: room for the wide tree + wordmark lockup */
    .pf-c-login__main-header,
    ak-flow-executor::part(branding) {
      padding-top: 1.5rem;
    }
    img.pf-c-brand {
      max-width: 18rem;
      max-height: 3.5rem;
    }

    /* Single-language space: hide the locale picker */
    ak-flow-executor::part(locale-select) {
      display: none;
    }
  CSS
  )
}
