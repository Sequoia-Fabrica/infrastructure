# authentik Terraform: brand & login experience

Infrastructure-as-code for the Sequoia Fabrica look and feel of
[login.sequoia.garden](https://login.sequoia.garden).

## What's managed

- **Brand** (`brands.tf`): title, tree + wordmark logo, favicon, brand-wide flow
  background, and the custom CSS that themes the login flow, the user library
  and the admin UI in the landing-page palette.
- **Authentication flow** (`flows.tf`): `sequoia-fabrica-authentication`, a
  branded copy of the default flow ("Welcome back to the garden") that binds
  the stock identification / MFA / login stages, so behaviour is unchanged.

- **Applications** (`applications.tf`), their **providers** (`providers.tf`:
  OAuth2 for Grafana, Chat and Immich; proxy via the embedded outpost for the
  rest) and the **access bindings** that restrict who sees them
  (`policies.tf`). Client secrets are never declared: the attribute is
  computed, so the imported state carries the live value.
- **Custom scope mapping** `immich_same_users` (`property_mappings.tf`).

Not managed here: groups and their membership (Multipass and the admin UI own
those), the two hand-made flows `seqfab-auth-flow` and
`sequoia-fabrica-member-enrollment-flow`, the embedded outpost, and global
system settings. `setup-env.sh` warns about any application, provider or
binding it finds on the instance that this workspace does not know about.

Brand assets referenced by media key (`sequoia_fabrica_lockup.svg`, etc.) are
deployed by ansible from `ansible/roles/sequoia_fabrica/files/authentik/` into
`/opt/authentik/data/media/public/`. Run `make ansible` (or
`--tags authentik`) before applying a change that introduces a new asset.

## Prerequisites

- Terraform >= 1.7
- SSH access to `nursery.cloudforest-perch.ts.net` (Tailscale) with sudo
- A superuser account in authentik

## Usage

No persistent state, no long-lived API token. Each session mints a 1-hour
token, discovers the live object IDs, and imports them fresh.

```bash
cd terraform/authentik/
source setup-env.sh            # uses $(whoami) as the authentik username
source setup-env.sh jof        # or pass it explicitly
terraform init
terraform plan
terraform apply
./cleanup-env.sh               # afterwards: revoke the token, drop generated imports
```

`setup-env.sh` writes `imports.generated.tf` (gitignored) containing `import`
blocks for whatever already exists: always the brand, and after the first
apply also the branded flow and its bindings. Nothing is hard-coded, so the
workspace works against a rebuilt instance too.

## Rolling back

The stock `default-authentication-flow` is never touched. To fall back to it,
set `flow_authentication` in `brands.tf` to a data source for that slug and
apply; the branded flow can be deleted afterwards or left in place.

## Editing the brand

| Want to change...          | Edit                                                   |
|----------------------------|--------------------------------------------------------|
| Colours, fonts, CSS        | `branding_custom_css` in `brands.tf`                   |
| Logo / favicon / background| the SVG/PNG in `ansible/.../files/authentik/`, then `make ansible` |
| Login page heading         | `title` on the flow in `flows.tf`                       |
| Footer links               | Admin UI -> System -> Settings (global, not per brand; not in Terraform yet) |

Palette and asset rules live in `documentation/brand-style-guide.md`.

## File layout

| File                  | Purpose                                                     |
|-----------------------|-------------------------------------------------------------|
| `setup-env.sh`        | Mint ephemeral token, discover IDs, write generated imports |
| `cleanup-env.sh`      | Revoke terraform tokens, remove generated imports           |
| `versions.tf`         | Terraform / provider constraints, provider config           |
| `variables.tf`        | Brand domain                                                |
| `data.tf`             | Read-only references to default flows and stages            |
| `brands.tf`           | Brand: logo, favicon, background, footer links, CSS         |
| `flows.tf`            | Branded authentication flow and stage bindings              |
| `imports.generated.tf`| Generated per session, gitignored                           |
