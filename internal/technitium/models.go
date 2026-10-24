package technitium

type BaseResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type DhcpScopeList struct {
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	StartingAddress  string `json:"startingAddress"`
	EndingAddress    string `json:"endingAddress"`
	SubnetMask       string `json:"subnetMask"`
	NetworkAddress   string `json:"networkAddress"`
	BroadcastAddress string `json:"broadcastAddress"`
	InterfaceAddress string `json:"interfaceAddress"`
}

// DhcpScope is the response of /api/dhcp/scopes/get.
type DhcpScope struct {
	Name                                 string              `json:"name"`
	StartingAddress                      string              `json:"startingAddress"`
	EndingAddress                        string              `json:"endingAddress"`
	SubnetMask                           string              `json:"subnetMask"`
	LeaseTimeDays                        int64               `json:"leaseTimeDays"`
	LeaseTimeHours                       int64               `json:"leaseTimeHours"`
	LeaseTimeMinutes                     int64               `json:"leaseTimeMinutes"`
	OfferDelayTime                       int64               `json:"offerDelayTime"`
	PingCheckEnabled                     bool                `json:"pingCheckEnabled"`
	PingCheckTimeout                     int64               `json:"pingCheckTimeout"`
	PingCheckRetries                     int64               `json:"pingCheckRetries"`
	DomainName                           string              `json:"domainName"`
	DomainSearchList                     []string            `json:"domainSearchList"`
	DnsUpdates                           bool                `json:"dnsUpdates"`
	DnsOverwriteForDynamicLease          bool                `json:"dnsOverwriteForDynamicLease"`
	DnsTtl                               int64               `json:"dnsTtl"`
	ServerAddress                        string              `json:"serverAddress"`
	ServerHostName                       string              `json:"serverHostName"`
	BootFileName                         string              `json:"bootFileName"`
	RouterAddress                        string              `json:"routerAddress"`
	UseThisDnsServer                     bool                `json:"useThisDnsServer"`
	DnsServers                           []string            `json:"dnsServers"`
	WinsServers                          []string            `json:"winsServers"`
	NtpServers                           []string            `json:"ntpServers"`
	NtpServerDomainNames                 []string            `json:"ntpServerDomainNames"`
	StaticRoutes                         []DhcpStaticRoute   `json:"staticRoutes"`
	VendorInfo                           []DhcpVendorInfo    `json:"vendorInfo"`
	CapwapAcIpAddresses                  []string            `json:"capwapAcIpAddresses"`
	TftpServerAddresses                  []string            `json:"tftpServerAddresses"`
	GenericOptions                       []DhcpGenericOption `json:"genericOptions"`
	Exclusions                           []Exclusion         `json:"exclusions"`
	ReservedLeases                       []DhcpReservedLease `json:"reservedLeases"`
	AllowOnlyReservedLeases              bool                `json:"allowOnlyReservedLeases"`
	BlockLocallyAdministeredMacAddresses bool                `json:"blockLocallyAdministeredMacAddresses"`
	IgnoreClientIdentifierOption         bool                `json:"ignoreClientIdentifierOption"`
}

type Exclusion struct {
	StartingAddress string `json:"startingAddress"`
	EndingAddress   string `json:"endingAddress"`
}

type DhcpStaticRoute struct {
	Destination string `json:"destination"`
	SubnetMask  string `json:"subnetMask"`
	Router      string `json:"router"`
}

type DhcpVendorInfo struct {
	Identifier  string `json:"identifier"`
	Information string `json:"information"`
}

type DhcpGenericOption struct {
	Code  int64  `json:"code"`
	Value string `json:"value"`
}

// DhcpReservedLease is a reserved lease; Name is the scope name (input only).
type DhcpReservedLease struct {
	Name            string `json:"-"`
	HardwareAddress string `json:"hardwareAddress"`
	IpAddress       string `json:"address"`
	HostName        string `json:"hostName"`
	Comments        string `json:"comments"`
}
