output "server_ip" {
  value = hcloud_server.server.ipv4_address
}

output "agent_ips" {
  value = { for name, agent in hcloud_server.agent : name => agent.ipv4_address }
}

output "kubeconfig_command" {
  value = "make kubeconfig ENV=${var.environment} HOST=${hcloud_server.server.ipv4_address}"
}
