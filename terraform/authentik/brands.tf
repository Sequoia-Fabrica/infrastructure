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

    /*
     * Theme-neutral tokens only. Anything that sets a surface colour MUST be
     * scoped to a colour scheme below: an unscoped light background at :root
     * outranks authentik's dark theme and leaves light text on a light page.
     */
    :root {
      /* Emerald primary: buttons, active nav, focus rings (light theme values) */
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

      /* Header bar is dark in both themes (white text), so this is safe here.
         The sidebar is deliberately left alone: its nav text is dark in the
         light theme and would vanish on emerald. */
      --pf-c-page__header--BackgroundColor: #022c22;
      --pf-c-nav__link--m-current--after--BorderColor: #10b981;
      --pf-c-nav__link--hover--after--BorderColor: #10b981;

      /* System font stack, matching the landing page (Tailwind default) */
      --pf-global--FontFamily--sans-serif:
        ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto,
        "Helvetica Neue", Arial, "Noto Sans", sans-serif;
    }

    /* Light theme: soft white-smoke surfaces */
    @media (prefers-color-scheme: light) {
      :root {
        --pf-global--BackgroundColor--100: #ffffff;
        --pf-global--BackgroundColor--200: #f7f4f3;
        --pf-global--BackgroundColor--light-300: #f7f4f3;
      }
    }

    /* Dark theme: deep emerald surfaces (authentik's own tokens) and
       brighter primaries so buttons and active items stay legible on them. */
    @media (prefers-color-scheme: dark) {
      :root {
        --ak-dark-background: #022c22;
        --ak-dark-background-darker: #011a14;
        --ak-dark-background-light: #05231c;
        --ak-dark-background-light-ish: #073a2e;
        --ak-dark-background-lighter: #0a4a3b;

        --ak-accent: #10b981;
        --pf-global--primary-color--100: #10b981;
        --pf-global--primary-color--200: #34d399;
        --pf-global--active-color--100: #34d399;
        --pf-global--active-color--300: #6ee7b7;
        --pf-global--link--Color: #fb923c;
        --pf-global--link--Color--hover: #fdba74;
      }
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

    /* Identification step: "Continue with Sequoia Fabrica Slack" first, the
       email/password form as the fallback beneath it.

       authentik adopts this stylesheet into every component's shadow root,
       so these rules run inside <ak-flow-card>. The card renders header,
       body (the form) and footer as sibling divs; the footer div only gets a
       <slot name="footer"> when something is slotted there, and in the flow
       UI the identification stage's login-sources fieldset is the only such
       thing. So "footer div contains slot[name=footer]" means "this card has
       login sources": we make the host a column flex container and move that
       footer above the body. :host() only takes a compound selector, so the
       scoping lives on the inner divs via :has(), not on the host. Other
       stages, and browsers without :has(), keep the stock order. */
    :host(ak-flow-card) {
      display: flex;
      flex-direction: column;
    }
    :host(ak-flow-card) .pf-c-login__main-header {
      order: -2;
    }
    :host(ak-flow-card) .pf-c-login__main-footer:has(> slot[name="footer"]) {
      order: -1;
      margin-block-start: 0;
      margin-block-end: 0;
    }
    :host(ak-flow-card) .pf-c-login__main-body:has(+ .pf-c-login__main-footer > slot[name="footer"])::before {
      content: "Or sign in with your email address";
      display: block;
      text-align: center;
      font-size: 0.875rem;
      color: var(--pf-global--Color--200);
      margin-block-end: 1rem;
    }

    /* The form's own submit button steps back to a secondary style so the
       promoted Slack button is the single primary action on the page. */
    :host(ak-stage-identification) form.pf-c-form .pf-c-button.pf-m-primary {
      background-color: transparent;
      color: var(--pf-global--primary-color--100);
      box-shadow: inset 0 0 0 1px var(--pf-global--primary-color--100);
    }
    :host(ak-stage-identification) form.pf-c-form .pf-c-button.pf-m-primary:hover {
      background-color: transparent;
      color: var(--pf-global--primary-color--200);
      box-shadow: inset 0 0 0 2px var(--pf-global--primary-color--200);
    }

    /* Single-language space: hide the locale picker */
    ak-flow-executor::part(locale-select) {
      display: none;
    }
  CSS
  )
}
