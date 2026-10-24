package provider

import (
	"net/netip"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Normalisers mirror what the server does to stored values.

// normName: domain names are case-insensitive, the server drops trailing
// dots and stores the root "." as "".
func normName(s string) string {
	return strings.ToLower(strings.TrimSuffix(s, "."))
}

func normIP(s string) string {
	if a, err := netip.ParseAddr(s); err == nil {
		return a.String()
	}
	return strings.ToLower(s)
}

func normHex(s string) string {
	return strings.ToUpper(strings.NewReplacer(":", "", " ", "").Replace(s))
}

func normFold(s string) string { return strings.ToLower(s) }

// Field constructors keep the spec table readable.

func keyStr(attrName, param, newParam, rdata string, norm func(string) string, desc string) recordField {
	return recordField{attr: attrName, param: param, newParam: newParam, rdata: rdata, kind: kString, key: true, del: true, norm: norm, desc: desc}
}

func keyInt(attrName, param, newParam, rdata, desc string) recordField {
	return recordField{attr: attrName, param: param, newParam: newParam, rdata: rdata, kind: kInt, key: true, del: true, desc: desc}
}

var addressFlags = []recordField{
	{attr: "ptr", param: "ptr", kind: kBool, optional: true, desc: "Also manage the matching PTR record in the reverse zone (applied on create and update only)."},
	{attr: "create_ptr_zone", param: "createPtrZone", kind: kBool, optional: true, desc: "Create the reverse zone for the PTR record if missing (with `ptr`)."},
	{attr: "update_svcb_hints", param: "updateSvcbHints", kind: kBool, optional: true, desc: "Update SVCB/HTTPS records with automatic hints that target this name."},
}

func addressSpec(typ, desc string) *recordSpec {
	return &recordSpec{
		typ:  typ,
		desc: desc,
		fields: append([]recordField{
			keyStr("ip_address", "ipAddress", "newIpAddress", "ipAddress", normIP, "IP address."),
		}, addressFlags...),
	}
}

func svcbSpec(typ, desc string) *recordSpec {
	return &recordSpec{
		typ:  typ,
		desc: desc,
		fields: []recordField{
			keyInt("priority", "svcPriority", "newSvcPriority", "svcPriority", "SvcPriority; 0 makes it an alias record."),
			keyStr("target_name", "svcTargetName", "newSvcTargetName", "svcTargetName", normName, "Target name; `.` means the owner name."),
			{attr: "params", param: "svcParams", newParam: "newSvcParams", rdata: "svcParams", kind: kStringMap, del: true, optional: true,
				desc: "SvcParams, e.g. `{ alpn = \"h2,h3\", port = \"8443\" }`. Hints filled by `auto_ipv4_hint`/`auto_ipv6_hint` are not tracked here."},
			{attr: "auto_ipv4_hint", param: "autoIpv4Hint", rdata: "autoIpv4Hint", kind: kBool, optional: true, desc: "Fill `ipv4hint` automatically from the target's A records."},
			{attr: "auto_ipv6_hint", param: "autoIpv6Hint", rdata: "autoIpv6Hint", kind: kBool, optional: true, desc: "Fill `ipv6hint` automatically from the target's AAAA records."},
		},
		fixRData: func(rdata map[string]any, _ map[string]attr.Value) {
			params, ok := rdata["svcParams"].(map[string]any)
			if !ok {
				return
			}
			cp := map[string]any{}
			for k, v := range params {
				if (k == "ipv4hint" && rdata["autoIpv4Hint"] == true) || (k == "ipv6hint" && rdata["autoIpv6Hint"] == true) {
					continue
				}
				cp[k] = v
			}
			rdata["svcParams"] = cp
		},
	}
}

var recordSpecs = []*recordSpec{
	addressSpec("A", "Manages an A (IPv4 address) record."),
	addressSpec("AAAA", "Manages an AAAA (IPv6 address) record."),
	{
		typ:  "NS",
		desc: "Manages an NS (name server) record.",
		fields: []recordField{
			keyStr("name_server", "nameServer", "newNameServer", "nameServer", normName, "Name server domain name."),
		},
	},
	{
		typ:       "CNAME",
		desc:      "Manages a CNAME (canonical name) record.",
		singleton: true,
		fields: []recordField{
			{attr: "cname", param: "cname", rdata: "cname", kind: kString, norm: normName, desc: "Canonical name the record points to."},
		},
	},
	{
		typ:  "PTR",
		desc: "Manages a PTR (pointer) record.",
		fields: []recordField{
			keyStr("ptr_name", "ptrName", "newPtrName", "ptrName", normName, "Domain name the pointer resolves to."),
		},
	},
	{
		typ:  "MX",
		desc: "Manages an MX (mail exchange) record.",
		fields: []recordField{
			keyInt("preference", "preference", "newPreference", "preference", "Preference; lower values are tried first."),
			keyStr("exchange", "exchange", "newExchange", "exchange", normName, "Mail server domain name."),
		},
	},
	{
		typ:  "TXT",
		desc: "Manages a TXT record. Text longer than 255 bytes is split into character-strings by the server.",
		fields: []recordField{
			keyStr("text", "text", "newText", "text", nil, "Text value."),
		},
		extraParams: func(params url.Values, op string) {
			params.Set("splitText", "false")
			if op == "update" {
				params.Set("newSplitText", "false")
			}
		},
	},
	{
		typ:  "RP",
		desc: "Manages an RP (responsible person) record.",
		fields: []recordField{
			keyStr("mailbox", "mailbox", "newMailbox", "mailbox", normFold, "Mailbox of the responsible person."),
			keyStr("txt_domain", "txtDomain", "newTxtDomain", "txtDomain", normName, "Domain name of a TXT record with more information."),
		},
	},
	{
		typ:  "SRV",
		desc: "Manages an SRV (service locator) record. `name` is the service label, e.g. `_sip._tcp`.",
		fields: []recordField{
			keyInt("priority", "priority", "newPriority", "priority", "Priority; lower values are tried first."),
			keyInt("weight", "weight", "newWeight", "weight", "Relative weight among records with the same priority."),
			keyInt("port", "port", "newPort", "port", "Service port."),
			keyStr("target", "target", "newTarget", "target", normName, "Target host name."),
		},
	},
	{
		typ:  "NAPTR",
		desc: "Manages a NAPTR (naming authority pointer) record.",
		fields: []recordField{
			keyInt("order", "naptrOrder", "naptrNewOrder", "order", "Order in which records are processed."),
			keyInt("preference", "naptrPreference", "naptrNewPreference", "preference", "Preference among records with the same order."),
			keyStr("flags", "naptrFlags", "naptrNewFlags", "flags", normFold, "Flags, e.g. `U`."),
			keyStr("services", "naptrServices", "naptrNewServices", "services", normFold, "Services, e.g. `E2U+sip`."),
			keyStr("regexp", "naptrRegexp", "naptrNewRegexp", "regexp", nil, "Substitution expression."),
			keyStr("replacement", "naptrReplacement", "naptrNewReplacement", "replacement", normName, "Replacement domain name, `.` for none."),
		},
	},
	{
		typ:       "DNAME",
		desc:      "Manages a DNAME (delegation name) record.",
		singleton: true,
		fields: []recordField{
			{attr: "dname", param: "dname", rdata: "dname", kind: kString, norm: normName, desc: "Target domain for the subtree."},
		},
	},
	{
		typ:  "DS",
		desc: "Manages a DS (delegation signer) record.",
		fields: []recordField{
			keyInt("key_tag", "keyTag", "newKeyTag", "keyTag", "Key tag of the referenced DNSKEY."),
			keyStr("algorithm", "algorithm", "newAlgorithm", "algorithm", normFold, "DNSSEC algorithm, e.g. `ECDSAP256SHA256`."),
			keyStr("digest_type", "digestType", "newDigestType", "digestType", normFold, "Digest type, e.g. `SHA256`."),
			keyStr("digest", "digest", "newDigest", "digest", normHex, "Digest as a hex string."),
		},
	},
	{
		typ:  "SSHFP",
		desc: "Manages an SSHFP (SSH fingerprint) record.",
		fields: []recordField{
			keyStr("algorithm", "sshfpAlgorithm", "newSshfpAlgorithm", "algorithm", normFold, "Key algorithm: `RSA`, `DSA`, `ECDSA`, `Ed25519` or `Ed448`."),
			keyStr("fingerprint_type", "sshfpFingerprintType", "newSshfpFingerprintType", "fingerprintType", normFold, "Fingerprint type: `SHA1` or `SHA256`."),
			keyStr("fingerprint", "sshfpFingerprint", "newSshfpFingerprint", "fingerprint", normHex, "Fingerprint as a hex string."),
		},
	},
	{
		typ:  "TLSA",
		desc: "Manages a TLSA (DANE) record. `name` is e.g. `_443._tcp`.",
		fields: []recordField{
			keyStr("certificate_usage", "tlsaCertificateUsage", "newTlsaCertificateUsage", "certificateUsage", normFold, "`PKIX-TA`, `PKIX-EE`, `DANE-TA` or `DANE-EE`."),
			keyStr("selector", "tlsaSelector", "newTlsaSelector", "selector", normFold, "`Cert` or `SPKI`."),
			keyStr("matching_type", "tlsaMatchingType", "newTlsaMatchingType", "matchingType", normFold, "`Full`, `SHA2-256` or `SHA2-512`."),
			keyStr("certificate_association_data", "tlsaCertificateAssociationData", "newTlsaCertificateAssociationData", "certificateAssociationData", normHex, "Certificate association data as a hex string."),
		},
	},
	svcbSpec("SVCB", "Manages an SVCB (service binding) record."),
	svcbSpec("HTTPS", "Manages an HTTPS (HTTPS service binding) record."),
	{
		typ:  "URI",
		desc: "Manages a URI record.",
		fields: []recordField{
			keyInt("priority", "uriPriority", "newUriPriority", "priority", "Priority; lower values are tried first."),
			keyInt("weight", "uriWeight", "newUriWeight", "weight", "Relative weight among records with the same priority."),
			keyStr("uri", "uri", "newUri", "uri", nil, "Target URI."),
		},
	},
	{
		typ:  "CAA",
		desc: "Manages a CAA (certification authority authorization) record.",
		fields: []recordField{
			keyInt("flags", "flags", "newFlags", "flags", "Flags; 128 marks the property critical."),
			keyStr("tag", "tag", "newTag", "tag", normFold, "Property tag: `issue`, `issuewild` or `iodef`."),
			keyStr("value", "value", "newValue", "value", nil, "Property value, e.g. `letsencrypt.org`."),
		},
	},
	{
		typ:  "ANAME",
		desc: "Manages an ANAME record (Technitium-specific CNAME flattening, usable at the zone apex).",
		fields: []recordField{
			keyStr("aname", "aname", "newAName", "aname", normName, "Domain name whose addresses are served."),
		},
	},
	{
		typ:   "FWD",
		desc:  "Manages an FWD record (Technitium-specific forwarder) in a conditional forwarder zone. The server ignores `ttl` for FWD records.",
		noTTL: true,
		fields: []recordField{
			{attr: "protocol", param: "protocol", newParam: "newProtocol", rdata: "protocol", kind: kString, key: true, del: true, optional: true, defStr: "Udp", norm: normFold,
				desc: "Transport: `Udp`, `Tcp`, `Tls`, `Https` or `Quic`."},
			keyStr("forwarder", "forwarder", "newForwarder", "forwarder", normFold, "Forwarder address, or `this-server`."),
			{attr: "priority", param: "forwarderPriority", rdata: "priority", kind: kInt, optional: true, desc: "Priority; lower values are queried first, equal priorities concurrently."},
			{attr: "dnssec_validation", param: "dnssecValidation", rdata: "dnssecValidation", kind: kBool, optional: true, desc: "Validate DNSSEC for forwarded responses."},
			{attr: "proxy_type", param: "proxyType", rdata: "proxyType", kind: kString, optional: true, defStr: "DefaultProxy", norm: normFold,
				desc: "`NoProxy`, `DefaultProxy`, `Http` or `Socks5`."},
			{attr: "proxy_address", param: "proxyAddress", rdata: "proxyAddress", kind: kString, optional: true, desc: "Proxy address when `proxy_type` is `Http` or `Socks5`."},
			{attr: "proxy_port", param: "proxyPort", rdata: "proxyPort", kind: kInt, optional: true, desc: "Proxy port."},
			{attr: "proxy_username", param: "proxyUsername", rdata: "proxyUsername", kind: kString, optional: true, desc: "Proxy username."},
			{attr: "proxy_password", param: "proxyPassword", kind: kString, optional: true, sens: true, desc: "Proxy password. Not readable from the server, so changes made outside Terraform are not detected."},
		},
	},
	{
		typ:       "APP",
		desc:      "Manages an APP record, which hands queries for the name to an installed DNS app.",
		singleton: true,
		fields: []recordField{
			{attr: "app_name", param: "appName", rdata: "appName", kind: kString, key: true, desc: "Installed app name, e.g. `Split Horizon`."},
			{attr: "class_path", param: "classPath", rdata: "classPath", kind: kString, key: true, desc: "App class path, e.g. `SplitHorizon.SimpleAddress`."},
			{attr: "record_data", param: "recordData", rdata: "data", kind: kString, optional: true, desc: "App-specific record data (often JSON)."},
		},
	},
}

func recordResources() []func() resource.Resource {
	out := make([]func() resource.Resource, 0, len(recordSpecs))
	for _, s := range recordSpecs {
		out = append(out, newRecordResource(s))
	}
	return out
}
