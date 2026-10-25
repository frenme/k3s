environment = "dev"

ssh_public_keys = [
  "ssh-ed25519 REPLACE_WITH_YOUR_PUBLIC_KEY",
]

admin_cidrs = ["REPLACE_WITH_YOUR_IP/32"]

server = {
  location    = "fsn1"
  server_type = "cpx21"
  region      = "eu"
}

nodes = {
  edge = {
    location    = "nbg1"
    server_type = "cpx11"
    region      = "eu"
  }
}

etcd_s3 = {
  endpoint = "s3.example.com"
  bucket   = "k3s-backups"
}
