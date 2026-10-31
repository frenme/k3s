environment = "prod"

ssh_public_keys = [
  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINO9QrhDNKwmSNXLzq7gnpfV4XtgRM+X2YnbR6QFnwid frenman@icloud.com",
]

server = {
  location    = "fsn1"
  server_type = "cpx42"
  backups     = true
}

nodes = {
  edge-eu = {
    location    = "nbg1"
    server_type = "cpx32"
  }
}
