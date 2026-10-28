variable "environment" {
  type = string
}

variable "ssh_public_keys" {
  type = list(string)
}

variable "image" {
  type    = string
  default = "ubuntu-24.04"
}

variable "server" {
  type = object({
    location    = string
    server_type = string
    protected   = optional(bool, true)
    backups     = optional(bool, false)
  })
}

variable "nodes" {
  type = map(object({
    location    = string
    server_type = string
    role        = optional(string, "edge")
    protected   = optional(bool)
    backups     = optional(bool, false)
  }))

  validation {
    condition     = alltrue([for node in var.nodes : contains(["core", "edge"], node.role)])
    error_message = "node role must be core or edge"
  }

  validation {
    condition     = anytrue([for node in var.nodes : node.role == "edge"])
    error_message = "at least one edge node is required to serve ingress"
  }
}
