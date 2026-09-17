# No secrets live here. The only input is the brand domain; everything that
# is instance-specific (object UUIDs to import) is discovered from the API by
# setup-env.sh and written to the gitignored imports.generated.tf.

variable "authentik_domain" {
  description = "Domain the brand matches on (also the public host of the instance)"
  type        = string
  default     = "login.sequoia.garden"
}
