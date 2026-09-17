#!/usr/bin/env bash
# -------------------------------------------------------------------
# Delete ephemeral Terraform tokens from authentik and drop the generated
# import file.
#
# Usage:
#   ./cleanup-env.sh           # expired terraform-* tokens only
#   ./cleanup-env.sh --all     # every terraform-* token, expired or not
# -------------------------------------------------------------------

set -euo pipefail

AUTHENTIK_SSH_HOST="nursery.cloudforest-perch.ts.net"
AUTHENTIK_CONTAINER="authentik-server-1"
DELETE_ALL="${1:-}"
_here="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"

FILTER="expires__lt=timezone.now(),"
[[ "${DELETE_ALL}" == "--all" ]] && FILTER=""

echo "==> Cleaning up Terraform tokens via ${AUTHENTIK_SSH_HOST}..."
DELETED=$(ssh "${AUTHENTIK_SSH_HOST}" \
  "sudo docker exec ${AUTHENTIK_CONTAINER} ak shell -c \"
from authentik.core.models import Token, TokenIntents
from django.utils import timezone
qs = Token.objects.filter(
    identifier__startswith='terraform-',
    intent=TokenIntents.INTENT_API,
    expiring=True,
    ${FILTER}
)
count = qs.count()
qs.delete()
print(count)
\"" 2>/dev/null | tail -1)
echo "    Deleted ${DELETED} token(s)."

rm -f "${_here}/imports.generated.tf"
unset AUTHENTIK_TOKEN AUTHENTIK_URL 2>/dev/null || true
echo "==> Done."
