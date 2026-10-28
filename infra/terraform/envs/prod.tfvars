environment = "prod"

ssh_public_keys = [
  "ssh-ed25519 REPLACE_WITH_YOUR_PUBLIC_KEY",
]

server = {
  location    = "fsn1"
  server_type = "cpx41"
  backups     = true
}

nodes = {
  edge-eu = {
    location    = "nbg1"
    server_type = "cpx21"
  }
}
