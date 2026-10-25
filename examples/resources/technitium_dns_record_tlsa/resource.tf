resource "technitium_dns_record_tlsa" "smtp" {
  zone                         = "example.com"
  name                         = "_25._tcp.mail"
  certificate_usage            = "DANE-EE"
  selector                     = "SPKI"
  matching_type                = "SHA2-256"
  certificate_association_data = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
}
