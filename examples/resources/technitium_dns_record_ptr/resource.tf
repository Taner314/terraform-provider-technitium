resource "technitium_dns_record_ptr" "nas" {
  zone     = "1.168.192.in-addr.arpa"
  name     = "10"
  ptr_name = "nas.example.com"
}
