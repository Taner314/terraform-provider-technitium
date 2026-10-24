package provider

import "github.com/hashicorp/terraform-plugin-framework/resource"

func str(attr, key, desc string) settingField {
	return settingField{attr: attr, key: key, kind: sStr, desc: desc}
}
func num(attr, key, desc string) settingField {
	return settingField{attr: attr, key: key, kind: sInt, desc: desc}
}
func flag(attr, key, desc string) settingField {
	return settingField{attr: attr, key: key, kind: sBool, desc: desc}
}
func list(attr, key, desc string) settingField {
	return settingField{attr: attr, key: key, kind: sStrList, desc: desc}
}
func secret(attr, key, desc string) settingField {
	return settingField{attr: attr, key: key, kind: sStr, desc: desc, sensitive: true, writeOnly: true}
}

var settingsSpecs = []*settingsSpec{
	{
		name: "general",
		desc: "Manages general DNS server settings: identity, listeners, record defaults, EDNS and network tuning.",
		fields: []settingField{
			str("dns_server_domain", "dnsServerDomain", "Primary domain name the server uses to identify itself."),
			list("local_end_points", "dnsServerLocalEndPoints", "Listen addresses with port, e.g. `0.0.0.0:53`, `[::]:53`."),
			list("ipv4_source_addresses", "dnsServerIPv4SourceAddresses", "IPv4 source addresses for outbound queries."),
			list("ipv6_source_addresses", "dnsServerIPv6SourceAddresses", "IPv6 source addresses for outbound queries."),
			num("default_record_ttl", "defaultRecordTtl", "Default TTL for new records."),
			num("default_ns_record_ttl", "defaultNsRecordTtl", "Default TTL for NS records."),
			num("default_soa_record_ttl", "defaultSoaRecordTtl", "Default TTL for SOA records."),
			str("default_responsible_person", "defaultResponsiblePerson", "Default SOA responsible person email; empty for the server default."),
			flag("use_soa_serial_date_scheme", "useSoaSerialDateScheme", "Default to the date-based SOA serial scheme for new zones."),
			num("min_soa_refresh", "minSoaRefresh", "Minimum SOA refresh honoured for secondary zones."),
			num("min_soa_retry", "minSoaRetry", "Minimum SOA retry honoured for secondary zones."),
			list("zone_transfer_allowed_networks", "zoneTransferAllowedNetworks", "Networks allowed to transfer all zones without TSIG."),
			list("notify_allowed_networks", "notifyAllowedNetworks", "Networks allowed to NOTIFY all secondary zones."),
			flag("enable_check_for_update", "dnsServerEnableCheckForUpdate", "Check for server updates."),
			flag("apps_enable_automatic_update", "dnsAppsEnableAutomaticUpdate", "Update installed DNS apps automatically."),
			str("ipv6_mode", "ipv6Mode", "IPv6 mode, e.g. `Disabled`, `Enabled` or `Preferred`."),
			flag("enable_udp_socket_pool", "enableUdpSocketPool", "Use a pool of random UDP ports for outbound queries."),
			{attr: "socket_pool_excluded_ports", key: "socketPoolExcludedPorts", kind: sIntList, desc: "Ports excluded from the UDP socket pool."},
			num("udp_payload_size", "udpPayloadSize", "EDNS UDP payload size."),
			flag("dnssec_validation", "dnssecValidation", "Validate DNSSEC for recursive and forwarded responses."),
			flag("edns_client_subnet", "eDnsClientSubnet", "Send EDNS Client Subnet (ECS) upstream."),
			num("edns_client_subnet_ipv4_prefix_length", "eDnsClientSubnetIPv4PrefixLength", "ECS IPv4 prefix length."),
			num("edns_client_subnet_ipv6_prefix_length", "eDnsClientSubnetIPv6PrefixLength", "ECS IPv6 prefix length."),
			str("edns_client_subnet_ipv4_override", "eDnsClientSubnetIpv4Override", "Fixed IPv4 network to send as ECS; empty to use the client's."),
			str("edns_client_subnet_ipv6_override", "eDnsClientSubnetIpv6Override", "Fixed IPv6 network to send as ECS; empty to use the client's."),
			flag("enable_edns_client_subnet_source_address", "enableEDnsClientSubnetSourceAddress", "Treat the ECS address in requests as the client address (for DNS proxies)."),
			num("client_timeout", "clientTimeout", "Milliseconds to wait for a resolution before answering ServerFailure."),
			num("tcp_send_timeout", "tcpSendTimeout", "TCP send timeout in milliseconds."),
			num("tcp_receive_timeout", "tcpReceiveTimeout", "TCP receive timeout in milliseconds."),
			num("quic_idle_timeout", "quicIdleTimeout", "QUIC idle timeout in milliseconds."),
			num("quic_max_inbound_streams", "quicMaxInboundStreams", "Maximum inbound QUIC streams per connection."),
			num("listen_backlog", "listenBacklog", "Maximum pending inbound connections."),
			num("udp_send_buffer_size_kb", "udpSendBufferSizeKB", "UDP socket send buffer size in KB."),
			num("udp_receive_buffer_size_kb", "udpReceiveBufferSizeKB", "UDP socket receive buffer size in KB."),
			num("max_concurrent_resolutions_per_core", "maxConcurrentResolutionsPerCore", "Concurrent resolutions per CPU core."),
		},
	},
	{
		name: "rate_limiting",
		desc: "Manages per-subnet query rate limits (queries per minute).",
		fields: []settingField{
			{attr: "ipv4_prefix_limits", key: "qpmPrefixLimitsIPv4", kind: sQpmList, desc: "IPv4 limits: objects with `prefix`, `udp_limit`, `tcp_limit`."},
			{attr: "ipv6_prefix_limits", key: "qpmPrefixLimitsIPv6", kind: sQpmList, desc: "IPv6 limits: objects with `prefix`, `udp_limit`, `tcp_limit`."},
			num("sample_minutes", "qpmLimitSampleMinutes", "Sample window in minutes."),
			num("udp_truncation_percentage", "qpmLimitUdpTruncationPercentage", "Percentage of over-limit UDP queries answered with TC instead of dropped."),
			list("bypass_list", "qpmLimitBypassList", "Addresses/networks exempt from rate limiting."),
		},
	},
	{
		name: "web_service",
		desc: "Manages the web console/API listener. Changing ports or TLS can cut off the provider's own connection; update the provider `host` accordingly.",
		fields: []settingField{
			list("local_addresses", "webServiceLocalAddresses", "Listen addresses."),
			num("http_port", "webServiceHttpPort", "HTTP port."),
			flag("enable_tls", "webServiceEnableTls", "Enable HTTPS."),
			flag("enable_http3", "webServiceEnableHttp3", "Enable HTTP/3."),
			flag("http_to_tls_redirect", "webServiceHttpToTlsRedirect", "Redirect HTTP to HTTPS."),
			flag("use_self_signed_tls_certificate", "webServiceUseSelfSignedTlsCertificate", "Generate a self-signed certificate."),
			num("tls_port", "webServiceTlsPort", "HTTPS port."),
			str("tls_certificate_path", "webServiceTlsCertificatePath", "Path to a PKCS #12 (.pfx) certificate on the server."),
			secret("tls_certificate_password", "webServiceTlsCertificatePassword", "Certificate password. Not readable from the server."),
			list("reverse_proxy_addresses", "webServiceReverseProxyAddresses", "Reverse proxies whose real-IP header is trusted."),
			str("real_ip_header", "webServiceRealIpHeader", "Header carrying the client IP from a reverse proxy."),
			flag("enable_http_unix_socket", "webServiceEnableHttpUnixSocket", "Listen on an HTTP Unix socket."),
			str("http_unix_socket", "webServiceHttpUnixSocket", "HTTP Unix socket path."),
			flag("enable_tls_unix_socket", "webServiceEnableTlsUnixSocket", "Listen on a TLS Unix socket."),
			str("tls_unix_socket", "webServiceTlsUnixSocket", "TLS Unix socket path."),
		},
	},
	{
		name: "protocols",
		desc: "Manages optional DNS transports: DNS-over-TLS/HTTPS/QUIC and PROXY-protocol listeners.",
		fields: []settingField{
			flag("enable_dns_over_udp_proxy", "enableDnsOverUdpProxy", "Accept DNS-over-UDP with the PROXY protocol."),
			num("dns_over_udp_proxy_port", "dnsOverUdpProxyPort", "DNS-over-UDP-PROXY port."),
			flag("enable_dns_over_tcp_proxy", "enableDnsOverTcpProxy", "Accept DNS-over-TCP with the PROXY protocol."),
			num("dns_over_tcp_proxy_port", "dnsOverTcpProxyPort", "DNS-over-TCP-PROXY port."),
			flag("enable_dns_over_http", "enableDnsOverHttp", "Accept plain-HTTP DoH behind a TLS-terminating proxy."),
			num("dns_over_http_port", "dnsOverHttpPort", "DNS-over-HTTP port."),
			flag("enable_dns_over_tls", "enableDnsOverTls", "Enable DNS-over-TLS."),
			num("dns_over_tls_port", "dnsOverTlsPort", "DNS-over-TLS port."),
			flag("enable_dns_over_https", "enableDnsOverHttps", "Enable DNS-over-HTTPS."),
			flag("enable_dns_over_http3", "enableDnsOverHttp3", "Enable DNS-over-HTTP/3."),
			num("dns_over_https_port", "dnsOverHttpsPort", "DNS-over-HTTPS port."),
			flag("enable_dns_over_quic", "enableDnsOverQuic", "Enable DNS-over-QUIC."),
			num("dns_over_quic_port", "dnsOverQuicPort", "DNS-over-QUIC port."),
			flag("enable_dns_over_http_help_redirect", "enableDnsOverHttpHelpRedirect", "Redirect browsers hitting the DoH endpoint to a help page."),
			list("reverse_proxy_network_acl", "dnsReverseProxyNetworkACL", "Proxies allowed to use the PROXY-protocol and DoH-over-HTTP listeners."),
			str("tls_certificate_path", "dnsTlsCertificatePath", "Path to a PKCS #12 (.pfx) certificate for DoT/DoH/DoQ."),
			secret("tls_certificate_password", "dnsTlsCertificatePassword", "Certificate password. Not readable from the server."),
			str("real_ip_header", "dnsOverHttpRealIpHeader", "Header carrying the client IP for DNS-over-HTTP."),
			flag("enable_dns_over_http_unix_socket", "enableDnsOverHttpUnixSocket", "Listen for DoH on an HTTP Unix socket."),
			str("dns_over_http_unix_socket", "dnsOverHttpUnixSocket", "DoH HTTP Unix socket path."),
			flag("enable_dns_over_https_unix_socket", "enableDnsOverHttpsUnixSocket", "Listen for DoH on an HTTPS Unix socket."),
			str("dns_over_https_unix_socket", "dnsOverHttpsUnixSocket", "DoH HTTPS Unix socket path."),
		},
	},
	{
		name: "recursion",
		desc: "Manages recursive resolution.",
		fields: []settingField{
			str("recursion", "recursion", "`Deny`, `Allow`, `AllowOnlyForPrivateNetworks` or `UseSpecifiedNetworkACL`."),
			list("network_acl", "recursionNetworkACL", "Ordered ACL when `recursion` is `UseSpecifiedNetworkACL`; prefix `!` to deny."),
			flag("randomize_name", "randomizeName", "Randomise query name case (0x20)."),
			flag("qname_minimization", "qnameMinimization", "Use QNAME minimisation."),
			flag("locally_served_dns_zones", "locallyServedDnsZones", "Answer RFC 6303 locally-served zones locally."),
			num("resolver_retries", "resolverRetries", "Retries per name server."),
			num("resolver_timeout", "resolverTimeout", "Timeout per query in milliseconds."),
			num("resolver_concurrency", "resolverConcurrency", "Name servers queried concurrently."),
			num("resolver_max_stack_count", "resolverMaxStackCount", "Maximum recursion depth."),
		},
	},
	{
		name: "cache",
		desc: "Manages the resolver cache.",
		fields: []settingField{
			flag("save_cache", "saveCache", "Persist the cache across restarts."),
			flag("serve_stale", "serveStale", "Serve expired records when upstreams fail (RFC 8767)."),
			num("serve_stale_ttl", "serveStaleTtl", "Seconds stale records are kept."),
			num("serve_stale_answer_ttl", "serveStaleAnswerTtl", "TTL used in stale answers."),
			num("serve_stale_reset_ttl", "serveStaleResetTtl", "TTL reset applied to stale records after an answer."),
			num("serve_stale_max_wait_time", "serveStaleMaxWaitTime", "Milliseconds to wait for a fresh answer before serving stale."),
			num("maximum_entries", "cacheMaximumEntries", "Maximum cache entries."),
			num("minimum_record_ttl", "cacheMinimumRecordTtl", "Minimum TTL applied to cached records."),
			num("maximum_record_ttl", "cacheMaximumRecordTtl", "Maximum TTL applied to cached records."),
			num("negative_record_ttl", "cacheNegativeRecordTtl", "TTL for negative answers."),
			num("failure_record_ttl", "cacheFailureRecordTtl", "TTL for resolution failures."),
			flag("enable_prefetch", "enableCachePrefetch", "Prefetch popular records before they expire."),
			num("prefetch_eligibility", "cachePrefetchEligibility", "Hits per hour needed for prefetch."),
			num("prefetch_trigger", "cachePrefetchTrigger", "Remaining TTL in seconds that triggers prefetch."),
		},
	},
	{
		name: "blocking",
		desc: "Manages ad/malware blocking and block list subscriptions.",
		fields: []settingField{
			flag("enable_blocking", "enableBlocking", "Enable blocking."),
			flag("allow_txt_blocking_report", "allowTxtBlockingReport", "Answer TXT queries for blocked names with a report."),
			list("bypass_list", "blockingBypassList", "Clients exempt from blocking."),
			str("blocking_type", "blockingType", "`AnyAddress`, `NxDomain` or `CustomAddress`."),
			num("blocking_answer_ttl", "blockingAnswerTtl", "TTL for blocked answers."),
			list("custom_blocking_addresses", "customBlockingAddresses", "Addresses returned when `blocking_type` is `CustomAddress`."),
			list("block_list_urls", "blockListUrls", "Block list URLs; prefix `!` for allow lists."),
			num("block_list_update_interval_hours", "blockListUpdateIntervalHours", "Block list refresh interval in hours."),
		},
	},
	{
		name: "proxy",
		desc: "Manages the outbound proxy used for recursion, forwarding and block list downloads.",
		fields: []settingField{
			{attr: "type", key: "proxyType", get: "proxy.type", kind: sStr, desc: "`None`, `Http` or `Socks5`."},
			{attr: "address", key: "proxyAddress", get: "proxy.address", kind: sStr, desc: "Proxy host."},
			{attr: "port", key: "proxyPort", get: "proxy.port", kind: sInt, desc: "Proxy port."},
			{attr: "username", key: "proxyUsername", get: "proxy.username", kind: sStr, desc: "Proxy username."},
			secret("password", "proxyPassword", "Proxy password. Not readable from the server."),
			{attr: "bypass", key: "proxyBypass", get: "proxy.bypass", kind: sStrList, desc: "Addresses, networks or names never sent through the proxy."},
		},
	},
	{
		name: "forwarders",
		desc: "Manages server-wide forwarders. With no forwarders the server resolves recursively.",
		fields: []settingField{
			list("forwarders", "forwarders", "Forwarder addresses, e.g. `1.1.1.1`, `dns.quad9.net (9.9.9.9:853)`, `https://cloudflare-dns.com/dns-query`."),
			str("protocol", "forwarderProtocol", "`Udp`, `Tcp`, `Tls`, `Https` or `Quic`."),
			flag("concurrent_forwarding", "concurrentForwarding", "Query forwarders concurrently."),
			num("retries", "forwarderRetries", "Retries per forwarder."),
			num("timeout", "forwarderTimeout", "Timeout per query in milliseconds."),
			num("concurrency", "forwarderConcurrency", "Forwarders queried concurrently."),
		},
	},
	{
		name: "logging",
		desc: "Manages logging and statistics retention.",
		fields: []settingField{
			flag("enable_logging", "enableLogging", "Enable logging."),
			str("logging_type", "loggingType", "`None`, `File`, `Console` or `FileAndConsole`."),
			flag("ignore_resolver_logs", "ignoreResolverLogs", "Don't log resolver errors."),
			flag("log_queries", "logQueries", "Log every query."),
			flag("no_stack_trace", "noStackTrace", "Omit stack traces from error logs."),
			flag("use_local_time", "useLocalTime", "Use local time instead of UTC in logs."),
			str("log_folder", "logFolder", "Log folder path on the server."),
			num("max_log_file_days", "maxLogFileDays", "Days to keep log files; 0 keeps forever."),
			flag("enable_in_memory_stats", "enableInMemoryStats", "Keep stats in memory only."),
			num("max_stat_file_days", "maxStatFileDays", "Days to keep stats files; 0 keeps forever."),
		},
	},
}

func settingsResources() []func() resource.Resource {
	out := make([]func() resource.Resource, 0, len(settingsSpecs))
	for _, s := range settingsSpecs {
		out = append(out, newSettingsResource(s))
	}
	return out
}
