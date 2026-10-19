terraform {
  required_version = ">= 1.6"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.70"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.9"
    }
    sops = {
      source  = "carlpett/sops"
      version = "~> 1.2"
    }
  }
}
