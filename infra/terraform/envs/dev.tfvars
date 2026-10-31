environment = "dev"

ssh_public_keys = [
  "ssh-ed25519 REPLACE_WITH_YOUR_PUBLIC_KEY",
]

server = {
  location    = "fsn1"
  server_type = "cpx22"
  protected   = false
}

nodes = {
  edge = {
    location    = "nbg1"
    server_type = "cx23"
  }
}
