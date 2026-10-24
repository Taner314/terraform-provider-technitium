# <zone>/<name>/<priority> <weight> <port> <target>
terraform import technitium_dns_record_srv.sip "example.com/_sip._tcp/10 5 5060 sip.example.com"
