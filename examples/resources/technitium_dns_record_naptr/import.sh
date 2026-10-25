# <zone>/<name>/<order> <preference> <flags> <services> <regexp> <replacement>
terraform import technitium_dns_record_naptr.sip "example.com/@/100 10 U E2U+sip !^.*$!sip:info@example.com! ."
