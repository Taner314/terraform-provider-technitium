# Values from `ssh-keygen -r server1.example.com`.
resource "technitium_dns_record_sshfp" "host" {
  zone             = "example.com"
  name             = "server1"
  algorithm        = "Ed25519"
  fingerprint_type = "SHA256"
  fingerprint      = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
}
