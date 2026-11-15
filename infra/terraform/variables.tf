variable "environment" {
  type = string
}

variable "ssh_public_keys" {
  type = list(string)
}

variable "protected" {
  type    = bool
  default = true
}

variable "api_sources" {
  description = "who may reach the Kubernetes API (6443); the nodes always may. Put your own address and the CI runner here to close the port to the world"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "server" {
  type = object({
    location    = string
    server_type = string
    edge        = optional(bool, false)
    backups     = optional(bool, false)
  })
}

variable "nodes" {
  type = map(object({
    location    = string
    server_type = string
    role        = optional(string, "edge")
  }))
  default = {}

  validation {
    condition     = alltrue([for node in var.nodes : contains(["core", "edge"], node.role)])
    error_message = "node role must be core or edge"
  }
}
