variable "environment" {
  type = string
}

variable "ssh_public_keys" {
  type = list(string)
}

variable "admin_cidrs" {
  type = list(string)
}

variable "image" {
  type    = string
  default = "ubuntu-24.04"
}

variable "k3s_version" {
  type    = string
  default = "v1.35.9+k3s1"
}

variable "server" {
  type = object({
    location    = string
    server_type = string
    region      = string
  })
}

variable "agents" {
  type = map(object({
    location    = string
    server_type = string
    region      = string
  }))

  validation {
    condition     = length(var.agents) > 0
    error_message = "at least one edge agent is required to serve ingress"
  }
}

variable "etcd_s3" {
  type = object({
    endpoint = string
    bucket   = string
  })
}
