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
}

type DhcpScope struct {
	Name             string      `json:"name"`
	StartingAddress  string      `json:"startingAddress"`
	EndingAddress    string      `json:"endingAddress"`
	SubnetMask       string      `json:"subnetMask"`
	RouterAddress    string      `json:"routerAddress,omitempty"`
	UseThisDnsServer bool        `json:"useThisDnsServer,omitempty"`
	DnsServers       []string    `json:"dnsServers,omitempty"`
	DomainName       string      `json:"DomainName,omitempty"`
	Exclusions       []Exclusion `json:"exclusions"`
}

type Exclusion struct {
	StartingAddress string `json:"startingAddress"`
	EndingAddress   string `json:"endingAddress"`
}

type DhcpReservedLease struct {
	Name            string `json:"name"`
	HardwareAddress string `json:"hardwareAddress"`
	IpAddress       string `json:"ipAddress"`
	HostName        string `json:"hostName,omitempty"`
	Comments        string `json:"comments,omitempty"`
}
