#!/usr/bin/env bash
# -------------------------------------------------------------------
# Source this script to set up ephemeral Terraform credentials for the
# Sequoia Fabrica authentik workspace.
#
#   1. SSHes to the nursery host and mints a 1-hour API token for your
#      authentik user via `ak shell` (no long-lived tokens anywhere).
#   2. Queries the API for the objects this workspace manages and writes
#      imports.generated.tf (gitignored) so a fresh `terraform plan` adopts
#      what already exists instead of trying to re-create it. There is no
#      persistent .tfstate; every session starts from scratch.
#
# Usage:
#   source setup-env.sh [authentik-username]
#
# The argument is your *authentik* username, defaulting to $(whoami). You
# need SSH access to nursery (Tailscale) and superuser rights in authentik.
# -------------------------------------------------------------------

set -euo pipefail

AUTHENTIK_HOST="login.sequoia.garden"
AUTHENTIK_SSH_HOST="nursery.cloudforest-perch.ts.net"
AUTHENTIK_CONTAINER="authentik-server-1"
AK_USER="${1:-$(whoami)}"
TOKEN_ID="terraform-$(date +%s)"
_here="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"

echo "==> Creating ephemeral API token for user '${AK_USER}' via ${AUTHENTIK_SSH_HOST}..."
_ak_err=$(mktemp)
AUTHENTIK_TOKEN=$(ssh "${AUTHENTIK_SSH_HOST}" \
  "sudo docker exec ${AUTHENTIK_CONTAINER} ak shell -c \"
from authentik.core.models import Token, TokenIntents, User
from django.utils import timezone
from datetime import timedelta
u = User.objects.get(username='${AK_USER}')
t = Token.objects.create(
    identifier='${TOKEN_ID}',
    user=u,
    intent=TokenIntents.INTENT_API,
    expiring=True,
    expires=timezone.now() + timedelta(hours=1),
    description='Ephemeral Terraform token',
)
print(t.key)
\"" 2>"${_ak_err}" | tail -1) || true

if [[ -z "${AUTHENTIK_TOKEN}" ]]; then
  echo "ERROR: Failed to create API token for authentik user '${AK_USER}'." >&2
  if grep -q "User matching query does not exist" "${_ak_err}"; then
    echo "       No authentik user named '${AK_USER}'. Pass it explicitly:" >&2
    echo "         source setup-env.sh <authentik-username>" >&2
  else
    echo "       Remote error (last lines):" >&2
    grep -vE '"logger": "authentik' "${_ak_err}" | tail -5 | sed 's/^/         /' >&2
  fi
  rm -f "${_ak_err}"
  return 1 2>/dev/null || exit 1
fi
rm -f "${_ak_err}"

export AUTHENTIK_URL="https://${AUTHENTIK_HOST}/"
export AUTHENTIK_TOKEN

# Slack app credentials (sources.tf) live encrypted in the ansible vault under
# authentik.sources.slack; decrypt them for this shell only. Nothing is
# written to disk and no *.tfvars file exists.
echo "==> Reading Slack app credentials from the ansible vault..."
_inv="${_here}/../../ansible/inventory"
_vault_pw="${HOME}/.sequoia_fabrica_ansible_vault"
_slack_json=$(ANSIBLE_DEPRECATION_WARNINGS=False ANSIBLE_LOCALHOST_WARNING=False \
  ansible -i "${_inv}" localhost -m debug -a "var=authentik.sources.slack" \
  --vault-password-file "${_vault_pw}" 2>/dev/null | sed -n '/=>/,$p' | sed '1s/^[^{]*//') || true
if ! _slack_env=$(python3 -c '
import json, shlex, sys
d = json.load(sys.stdin).get("authentik.sources.slack")
if not isinstance(d, dict) or not all(d.get(k) for k in ("client_id", "client_secret", "team_id")):
    sys.exit("authentik.sources.slack.{client_id,client_secret,team_id} must all be set in group_vars")
print("export TF_VAR_slack_client_id=" + shlex.quote(str(d["client_id"])))
print("export TF_VAR_slack_client_secret=" + shlex.quote(str(d["client_secret"])))
print("export TF_VAR_slack_team_id=" + shlex.quote(str(d.get("team_id") or "")))
' <<<"${_slack_json}" 2>&1); then
  echo "ERROR: could not read the Slack app credentials: ${_slack_env}" >&2
  echo "       Add them with:" >&2
  echo "         ansible-vault encrypt_string --vault-password-file ${_vault_pw} --stdin-name client_secret" >&2
  echo "       and paste the result under authentik.sources.slack in ansible/inventory/group_vars/all.yml" >&2
  unset AUTHENTIK_TOKEN AUTHENTIK_URL
  return 1 2>/dev/null || exit 1
fi
eval "${_slack_env}"
unset _slack_json _slack_env
echo "    client_id=${TF_VAR_slack_client_id} team_id=${TF_VAR_slack_team_id}"

echo "==> Discovering existing objects and writing imports.generated.tf..."
_rc=0
python3 - "${AUTHENTIK_URL}" "${AUTHENTIK_TOKEN}" "${AUTHENTIK_HOST}" "${_here}/imports.generated.tf" <<'PY' || _rc=$?
import json, sys, urllib.parse, urllib.request

base, token, domain, out_path = sys.argv[1:5]

# login.sequoia.garden sits behind a Cloudflare tunnel whose browser-integrity
# check rejects Python's default User-Agent (error 1010). Terraform's Go
# client is allowed, so we only need to identify ourselves sensibly here.
HEADERS = {
    "Authorization": f"Bearer {token}",
    "Accept": "application/json",
    "User-Agent": "sequoia-fabrica-infrastructure/terraform-authentik setup-env.sh",
}

def get(path, **params):
    url = f"{base}api/v3/{path}?{urllib.parse.urlencode(params)}" if params else f"{base}api/v3/{path}"
    req = urllib.request.Request(url, headers=HEADERS)
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        body = e.read().decode(errors="replace")[:300]
        if e.code == 403 and "1010" in body:
            sys.exit(f"Cloudflare blocked the request to {url} (error 1010, User-Agent check)")
        if e.code in (401, 403):
            sys.exit(f"HTTP {e.code} from {url}: the token's user probably lacks superuser/API rights.\n{body}")
        sys.exit(f"HTTP {e.code} from {url}: {body}")

blocks = []
def imp(to, ident):
    blocks.append(f'import {{\n  to = {to}\n  id = "{ident}"\n}}\n')

# Brand: match on domain; fall back to the default brand.
brands = get("core/brands/").get("results", [])
brand = next((b for b in brands if b["domain"] == domain), None) or next((b for b in brands if b["default"]), None)
if not brand:
    sys.exit(f"no brand found for {domain} and no default brand exists")
imp("authentik_brand.sequoia_fabrica", brand["brand_uuid"])
print(f"    brand      {brand['brand_uuid']}  domain={brand['domain']} title={brand['branding_title']!r}")

# Branded flow + its stage bindings: only present after the first apply.
flow_slug = "sequoia-fabrica-authentication"
flows = get("flows/instances/", slug=flow_slug).get("results", [])
if flows:
    flow = flows[0]
    imp("authentik_flow.sequoia_fabrica_authentication", flow_slug)
    print(f"    flow       {flow['pk']}  slug={flow_slug}")
    bindings = get("flows/bindings/", target=flow["pk"], ordering="order").get("results", [])
    names = {
        # Bound the stock stage until the Slack source landed, our own since.
        "default-authentication-identification": "sequoia_fabrica_auth_identification",
        "sequoia-fabrica-authentication-identification": "sequoia_fabrica_auth_identification",
        "default-authentication-mfa-validation": "sequoia_fabrica_auth_mfa_validation",
        "default-authentication-login": "sequoia_fabrica_auth_login",
    }
    for b in bindings:
        res = names.get(b["stage_obj"]["name"])
        if res:
            imp(f"authentik_flow_stage_binding.{res}", b["pk"])
            print(f"    binding    {b['pk']}  order={b['order']} stage={b['stage_obj']['name']}")
        else:
            print(f"    WARNING: unmanaged binding {b['pk']} (stage {b['stage_obj']['name']}) on {flow_slug}")
else:
    print(f"    flow       (not created yet: {flow_slug}; first apply will create it)")

# Applications, their providers and access bindings (applications.tf,
# providers.tf, policies.tf). Keyed by the live slug / provider name.
APPS = {  # slug -> terraform resource name
    "adminer": "adminer", "chat": "chat", "etherpad": "etherpad", "frigate": "frigate",
    "grafana": "grafana", "immich": "immich", "multipass": "multipass",
    "multipass-test": "multipass_test", "uptime-kuma": "uptime_kuma", "utilities": "utilities",
}
PROVIDERS = {  # provider name -> (resource type, terraform resource name)
    "Grafana": ("authentik_provider_oauth2", "grafana"),
    "Provider for Chat": ("authentik_provider_oauth2", "chat"),
    "Provider for Immich": ("authentik_provider_oauth2", "immich"),
    "Provider for Adminer": ("authentik_provider_proxy", "adminer"),
    "Provider for Etherpad": ("authentik_provider_proxy", "etherpad"),
    "Provider for Frigate": ("authentik_provider_proxy", "frigate"),
    "Provider for Multipass": ("authentik_provider_proxy", "multipass"),
    "Provider for multipass-test": ("authentik_provider_proxy", "multipass_test"),
    "Provider for Uptime Kuma": ("authentik_provider_proxy", "uptime_kuma"),
    "Provider for utilities": ("authentik_provider_proxy", "utilities"),
}
BINDINGS = {  # app slug -> terraform resource name of its single order-0 binding
    "adminer": "adminer_authentik_admins",
    "frigate": "frigate_security_system_operators",
    "utilities": "utilities_members",
    "multipass-test": "multipass_test_jof",
}
SCOPES = {"immich_same_users": "immich_same_users"}

apps = {a["slug"]: a for a in get("core/applications/", page_size=200).get("results", [])}
for slug, res in APPS.items():
    if slug in apps:
        imp(f"authentik_application.{res}", slug)
    else:
        print(f"    WARNING: application {slug} not found on the instance")
for slug in apps.keys() - APPS.keys():
    print(f"    (unmanaged) application: {slug}")

providers = {p["name"]: p for p in get("providers/all/", page_size=200).get("results", [])}
for name, (rtype, res) in PROVIDERS.items():
    if name in providers:
        imp(f"{rtype}.{res}", str(providers[name]["pk"]))
    else:
        print(f"    WARNING: provider {name!r} not found on the instance")
for name in providers.keys() - PROVIDERS.keys():
    print(f"    (unmanaged) provider: {name!r}")

for slug, res in BINDINGS.items():
    if slug not in apps:
        continue
    bs = get("policies/bindings/", target=apps[slug]["pk"], ordering="order").get("results", [])
    if bs:
        imp(f"authentik_policy_binding.{res}", bs[0]["pk"])
        for extra in bs[1:]:
            print(f"    WARNING: unmanaged binding {extra['pk']} (order {extra['order']}) on {slug}")
for slug, a in apps.items():
    if slug in APPS and slug not in BINDINGS:
        bs = get("policies/bindings/", target=a["pk"]).get("results", [])
        for b in bs:
            print(f"    WARNING: unmanaged binding {b['pk']} on {slug}")

for name, res in SCOPES.items():
    ms = get("propertymappings/provider/scope/", name=name).get("results", [])
    if ms:
        imp(f"authentik_property_mapping_provider_scope.{res}", ms[0]["pk"])
    else:
        print(f"    WARNING: scope mapping {name!r} not found on the instance")

print(f"    apps/providers/bindings/scopes: {len(APPS)}/{len(PROVIDERS)}/{len(BINDINGS)}/{len(SCOPES)} mapped")

# --- Slack source and its flows (sources.tf) ---------------------------
# Everything here is created by the first apply; before that nothing is found
# and nothing is imported. Objects are looked up by the names in sources.tf.

def first(path, **params):
    rs = get(path, **params).get("results", [])
    return rs[0] if rs else None

def imp_named(to, path, key, value, pk="pk"):
    obj = first(path, **{key: value})
    if obj:
        imp(to, str(obj[pk]))
    return obj

n_slack = 0
# The provider keys OAuth sources by slug, not by pk.
src = imp_named("authentik_source_oauth.slack", "sources/oauth/", "slug", "slack", pk="slug")
if src:
    n_slack += 1
    print(f"    slack      source {src['pk']} enrollment_flow={'set' if src.get('enrollment_flow') else 'none'}")
for to, path, key, value in [
    ("authentik_property_mapping_source_oauth.slack", "propertymappings/source/oauth/", "name", "Slack: profile and workspace identity"),
    ("authentik_group.slack_community", "core/groups/", "name", "Slack Community"),
    ("authentik_policy_expression.slack_if_sso", "policies/expression/", "name", "slack-source-if-sso"),
    ("authentik_policy_expression.slack_team_gate", "policies/expression/", "name", "slack-source-workspace-gate"),
    ("authentik_policy_expression.slack_if_no_username", "policies/expression/", "name", "slack-source-enrollment-if-no-username"),
    ("authentik_stage_prompt_field.slack_username", "stages/prompt/prompts/", "name", "slack-source-enrollment-field-username"),
    ("authentik_stage_prompt.slack_enrollment", "stages/prompt/stages/", "name", "slack-source-enrollment-prompt"),
    ("authentik_stage_user_write.slack_enrollment", "stages/user_write/", "name", "slack-source-enrollment-write"),
    ("authentik_stage_identification.sequoia_fabrica", "stages/identification/", "name", "sequoia-fabrica-authentication-identification"),
]:
    if imp_named(to, path, key, value):
        n_slack += 1

SLACK_FLOWS = {  # slug -> (flow resource, {stage name: binding resource}, {policy name: flow policy binding resource})
    "sequoia-fabrica-slack-enrollment": (
        "authentik_flow.slack_enrollment",
        {
            "slack-source-enrollment-prompt": "slack_enrollment_prompt",
            "slack-source-enrollment-write": "slack_enrollment_write",
            "default-source-enrollment-login": "slack_enrollment_login",
        },
        {
            "slack-source-if-sso": "slack_enrollment_if_sso",
            "slack-source-workspace-gate": "slack_enrollment_team_gate",
        },
    ),
    "sequoia-fabrica-slack-authentication": (
        "authentik_flow.slack_authentication",
        {"default-source-authentication-login": "slack_authentication_login"},
        {
            "slack-source-if-sso": "slack_authentication_if_sso",
            "slack-source-workspace-gate": "slack_authentication_team_gate",
        },
    ),
}
# The list endpoint's ?target= filter only accepts some binding-model types
# (a flow stage binding is rejected with HTTP 400), so index every policy
# binding once and match the target client-side.
policy_bindings_by_target = {}
for pb in get("policies/bindings/", page_size=500).get("results", []):
    policy_bindings_by_target.setdefault(str(pb["target"]), []).append(pb)

for slug, (flow_res, stage_names, policy_names) in SLACK_FLOWS.items():
    fl = first("flows/instances/", slug=slug)
    if not fl:
        continue
    n_slack += 1
    imp(flow_res, slug)
    for b in get("flows/bindings/", target=fl["pk"], ordering="order").get("results", []):
        res = stage_names.get(b["stage_obj"]["name"])
        if not res:
            print(f"    WARNING: unmanaged binding {b['pk']} (stage {b['stage_obj']['name']}) on {slug}")
            continue
        imp(f"authentik_flow_stage_binding.{res}", b["pk"])
        if res == "slack_enrollment_prompt":
            for pb in policy_bindings_by_target.get(str(b["pk"]), []):
                imp("authentik_policy_binding.slack_enrollment_prompt_if_no_username", pb["pk"])
    for pb in policy_bindings_by_target.get(str(fl["pk"]), []):
        pname = (pb.get("policy_obj") or {}).get("name")
        res = policy_names.get(pname)
        if res:
            imp(f"authentik_policy_binding.{res}", pb["pk"])
        else:
            print(f"    WARNING: unmanaged policy binding {pb['pk']} ({pname}) on {slug}")
print(f"    slack      {n_slack} object(s) found" if n_slack else "    slack      (nothing created yet; first apply creates the source and flows)")

# The stock identification stage is what the branded flow bound before the
# Slack source; our replacement copies its shape. Show anything configured on
# the stock stage that sources.tf may need to mirror.
ident = first("stages/identification/", name="default-authentication-identification")
if ident:
    extras = {k: ident.get(k) for k in ("recovery_flow", "enrollment_flow", "passwordless_flow", "captcha_stage", "sources")
              if ident.get(k)}
    print(f"    default identification stage: user_fields={ident.get('user_fields')} extras={extras or 'none'}")

header = (
    "# GENERATED by setup-env.sh -- do not edit, do not commit.\n"
    "# Maps objects that already exist on the instance to Terraform addresses.\n\n"
)
with open(out_path, "w") as f:
    f.write(header + "\n".join(blocks))
PY
if [[ ${_rc} -ne 0 ]]; then
  echo "ERROR: discovery failed (see above); not exporting credentials." >&2
  unset AUTHENTIK_TOKEN AUTHENTIK_URL TF_VAR_slack_client_id TF_VAR_slack_client_secret TF_VAR_slack_team_id
  return 1 2>/dev/null || exit 1
fi

echo "==> Ready. Token: ${TOKEN_ID} (user: ${AK_USER}, expires in 1h)"
echo "    AUTHENTIK_URL=${AUTHENTIK_URL}"
echo "    Run: terraform init && terraform plan"
