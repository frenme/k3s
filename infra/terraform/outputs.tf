output "server_ip" {
  value = hcloud_server.server.ipv4_address
}

output "node_ips" {
  value = { for name, node in hcloud_server.node : name => node.ipv4_address }
}

output "kubeconfig_command" {
  value = "make kubeconfig ENV=${var.environment} HOST=${hcloud_server.server.ipv4_address}"
}
