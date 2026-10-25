# Delegate lab.example.com to other name servers.
resource "technitium_dns_record_ns" "lab" {
  for_each    = toset(["ns1.lab.example.com", "ns2.lab.example.com"])
  zone        = "example.com"
  name        = "lab"
  name_server = each.value
}
