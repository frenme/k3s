data "sops_file" "secrets" {
  source_file = "${path.module}/../deploy/environments/${var.environment}.secrets.yaml"
}

locals {
  name = "k3s-${var.environment}"
  deploy = merge(
    yamldecode(file("${path.module}/../deploy/environments/common.yaml")),
    yamldecode(file("${path.module}/../deploy/environments/${var.environment}.yaml")),
  )
  etcd_s3     = try(local.deploy.backups, false)
  web_sources = try(local.deploy.cloudflareOnly, false) ? concat(local.deploy.cloudflare.ipv4, local.deploy.cloudflare.ipv6) : ["0.0.0.0/0", "::/0"]
  secrets     = yamldecode(data.sops_file.secrets.raw).secrets
  registry    = try(local.secrets.registry, null)
  labels      = { cluster = local.name }
}

provider "hcloud" {
  token = local.secrets.hcloud.token
}

resource "random_password" "token" {
  length  = 48
  special = false
}

resource "random_password" "agent_token" {
  length  = 48
  special = false
}

resource "hcloud_ssh_key" "admin" {
  for_each   = { for i, key in var.ssh_public_keys : tostring(i) => key }
  name       = "${local.name}-${each.key}"
  public_key = each.value
  labels     = local.labels
}

resource "hcloud_firewall" "base" {
  name   = "${local.name}-base"
  labels = local.labels

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "6443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_firewall" "web" {
  name   = "${local.name}-web"
  labels = local.labels

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = local.web_sources
  }

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = local.web_sources
  }
}

resource "hcloud_placement_group" "core" {
  name   = "${local.name}-core"
  type   = "spread"
  labels = local.labels
}

resource "hcloud_server" "server" {
  name         = "${local.name}-server"
  location     = var.server.location
  server_type  = var.server.server_type
  image        = var.image
  ssh_keys     = [for key in hcloud_ssh_key.admin : key.id]
  firewall_ids = [hcloud_firewall.base.id]
  labels       = merge(local.labels, { role = "core" })

  ignore_remote_firewall_ids = true
  placement_group_id         = hcloud_placement_group.core.id
  backups                    = var.server.backups
  delete_protection          = var.server.protected
  rebuild_protection         = var.server.protected

  user_data = templatefile("${path.module}/templates/server.yaml.tftpl", {
    token         = random_password.token.result
    agent_token   = random_password.agent_token.result
    k3s_version   = local.deploy.k3sVersion
    etcd_s3       = local.etcd_s3
    etcd_endpoint = try(local.deploy.s3.endpoint, "")
    etcd_bucket   = try(local.deploy.s3.bucket, "")
    etcd_folder   = "${local.name}/etcd"
    s3_access_key = try(local.secrets.s3.accessKeyId, "")
    s3_secret_key = try(local.secrets.s3.secretAccessKey, "")
    registry      = local.registry
    coredns       = file("${path.module}/../k3s/coredns.yaml")
  })

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }

  lifecycle {
    ignore_changes = [user_data, image, ssh_keys]
  }
}

resource "hcloud_server" "node" {
  for_each     = var.nodes
  name         = "${local.name}-${each.key}"
  location     = each.value.location
  server_type  = each.value.server_type
  image        = var.image
  ssh_keys     = [for key in hcloud_ssh_key.admin : key.id]
  firewall_ids = each.value.role == "edge" ? [hcloud_firewall.base.id, hcloud_firewall.web.id] : [hcloud_firewall.base.id]
  labels       = merge(local.labels, { role = each.value.role })

  ignore_remote_firewall_ids = true
  placement_group_id         = each.value.role == "core" ? hcloud_placement_group.core.id : null
  backups                    = each.value.backups
  delete_protection          = coalesce(each.value.protected, each.value.role == "core")
  rebuild_protection         = coalesce(each.value.protected, each.value.role == "core")

  user_data = templatefile("${path.module}/templates/agent.yaml.tftpl", {
    token       = random_password.agent_token.result
    k3s_version = local.deploy.k3sVersion
    server_ip   = hcloud_server.server.ipv4_address
    role        = each.value.role
    registry    = local.registry
  })

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }

  lifecycle {
    ignore_changes = [user_data, image, ssh_keys]
  }
}

locals {
  node_cidrs = concat(
    ["${hcloud_server.server.ipv4_address}/32"],
    [for node in hcloud_server.node : "${node.ipv4_address}/32"],
  )
}

resource "hcloud_firewall" "nodes" {
  name   = "${local.name}-nodes"
  labels = local.labels

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "10250"
    source_ips = local.node_cidrs
  }

  rule {
    direction  = "in"
    protocol   = "udp"
    port       = "51820-51821"
    source_ips = local.node_cidrs
  }
}

resource "hcloud_firewall_attachment" "nodes" {
  firewall_id = hcloud_firewall.nodes.id
  server_ids  = concat([hcloud_server.server.id], [for node in hcloud_server.node : node.id])
}
