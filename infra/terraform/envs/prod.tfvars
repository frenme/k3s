environment = "prod"

ssh_public_keys = [
  "ssh-ed25519 REPLACE_WITH_YOUR_PUBLIC_KEY",
]

admin_cidrs = ["REPLACE_WITH_YOUR_IP/32"]

server = {
  location    = "fsn1"
  server_type = "cpx41"
  region      = "eu"
}

agents = {
  asia = {
    location    = "sin"
    server_type = "cpx21"
    region      = "asia"
  }
  us = {
    location    = "ash"
    server_type = "cpx21"
    region      = "us"
  }
}

etcd_s3 = {
  endpoint = "s3.example.com"
  bucket   = "k3s-backups"
}
