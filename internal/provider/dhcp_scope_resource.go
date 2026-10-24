package provider

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &dhcpScopeResource{}
	_ resource.ResourceWithImportState = &dhcpScopeResource{}
)

func NewDhcpScopeResource() resource.Resource {
	return &dhcpScopeResource{}
}

type dhcpScopeResource struct {
	client *technitium.Client
}

// dhcpScopeModel: "options" (DNS servers, routes, ...) are authoritative and
// default to empty, so removing them from config clears them. Server-tuned
// knobs (lease time, ping check, ...) are Optional+Computed and follow the
// server when unset. Reserved leases are managed by
// technitium_dhcp_reserved_lease and never touched here.
type dhcpScopeModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	StartingAddress types.String `tfsdk:"starting_address"`
	EndingAddress   types.String `tfsdk:"ending_address"`
	SubnetMask      types.String `tfsdk:"subnet_mask"`

	LeaseTimeDays                        types.Int64 `tfsdk:"lease_time_days"`
	LeaseTimeHours                       types.Int64 `tfsdk:"lease_time_hours"`
	LeaseTimeMinutes                     types.Int64 `tfsdk:"lease_time_minutes"`
	OfferDelayTime                       types.Int64 `tfsdk:"offer_delay_time"`
	PingCheckEnabled                     types.Bool  `tfsdk:"ping_check_enabled"`
	PingCheckTimeout                     types.Int64 `tfsdk:"ping_check_timeout"`
	PingCheckRetries                     types.Int64 `tfsdk:"ping_check_retries"`
	DnsUpdates                           types.Bool  `tfsdk:"dns_updates"`
	DnsOverwriteForDynamicLease          types.Bool  `tfsdk:"dns_overwrite_for_dynamic_lease"`
	DnsTtl                               types.Int64 `tfsdk:"dns_ttl"`
	UseThisDnsServer                     types.Bool  `tfsdk:"use_this_dns_server"`
	AllowOnlyReservedLeases              types.Bool  `tfsdk:"allow_only_reserved_leases"`
	BlockLocallyAdministeredMacAddresses types.Bool  `tfsdk:"block_locally_administered_mac_addresses"`
	IgnoreClientIdentifierOption         types.Bool  `tfsdk:"ignore_client_identifier_option"`

	RouterAddress       types.String `tfsdk:"router_address"`
	DomainName          types.String `tfsdk:"domain_name"`
	DomainSearchList    types.List   `tfsdk:"domain_search_list"`
	DnsServers          types.List   `tfsdk:"dns_servers"`
	WinsServers         types.List   `tfsdk:"wins_servers"`
	NtpServers          types.List   `tfsdk:"ntp_servers"`
	ServerAddress       types.String `tfsdk:"server_address"`
	ServerHostName      types.String `tfsdk:"server_host_name"`
	BootFileName        types.String `tfsdk:"boot_file_name"`
	CapwapAcIpAddresses types.List   `tfsdk:"capwap_ac_ip_addresses"`
	TftpServerAddresses types.List   `tfsdk:"tftp_server_addresses"`
	StaticRoutes        types.List   `tfsdk:"static_routes"`
	VendorInfo          types.List   `tfsdk:"vendor_info"`
	GenericOptions      types.List   `tfsdk:"generic_options"`
	Exclusions          types.List   `tfsdk:"exclusions"`
}

var (
	exclusionType   = types.ObjectType{AttrTypes: map[string]attr.Type{"starting_address": types.StringType, "ending_address": types.StringType}}
	staticRouteType = types.ObjectType{AttrTypes: map[string]attr.Type{"destination": types.StringType, "subnet_mask": types.StringType, "router": types.StringType}}
	vendorInfoType  = types.ObjectType{AttrTypes: map[string]attr.Type{"identifier": types.StringType, "information": types.StringType}}
	genericOptType  = types.ObjectType{AttrTypes: map[string]attr.Type{"code": types.Int64Type, "value": types.StringType}}
)

func (r *dhcpScopeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *dhcpScopeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_scope"
}

func (r *dhcpScopeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	tunedInt := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Optional: true, Computed: true, Description: desc,
			PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}
	}
	tunedBool := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Optional: true, Computed: true, Description: desc,
			PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}
	}
	optStr := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Description: desc}
	}
	optList := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: desc,
			Default: listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{}))}
	}
	objList := func(t types.ObjectType, desc string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, Computed: true, ElementType: t, Description: desc,
			Default: listdefault.StaticValue(types.ListValueMust(t, []attr.Value{}))}
	}

	resp.Schema = schema.Schema{
		Description: "Manages a DHCP scope. Reserved leases are managed with `technitium_dhcp_reserved_lease` and are left untouched here.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Scope name.",
				PlanModifiers: []planmodifier.String{copyFrom("name")}},
			"name": schema.StringAttribute{Required: true, Description: "Scope name. Changing it renames the scope in place."},
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Whether the scope serves leases. The server needs an interface address in the subnet to answer clients."},
			"starting_address": schema.StringAttribute{Required: true, Description: "First address of the pool."},
			"ending_address":   schema.StringAttribute{Required: true, Description: "Last address of the pool."},
			"subnet_mask":      schema.StringAttribute{Required: true, Description: "Subnet mask, e.g. `255.255.255.0`."},

			"lease_time_days":                          tunedInt("Lease time, days part. Server default: 1 day."),
			"lease_time_hours":                         tunedInt("Lease time, hours part."),
			"lease_time_minutes":                       tunedInt("Lease time, minutes part."),
			"offer_delay_time":                         tunedInt("Milliseconds to delay DHCPOFFER (for standby servers)."),
			"ping_check_enabled":                       tunedBool("Ping an address before offering it."),
			"ping_check_timeout":                       tunedInt("Ping check timeout in milliseconds."),
			"ping_check_retries":                       tunedInt("Ping check attempts."),
			"dns_updates":                              tunedBool("Create forward/reverse DNS records for leases (needs `domain_name`)."),
			"dns_overwrite_for_dynamic_lease":          tunedBool("Let dynamic leases overwrite an existing A record with the same name."),
			"dns_ttl":                                  tunedInt("TTL of DNS records created for leases."),
			"use_this_dns_server":                      tunedBool("Hand out this server as DNS server; `dns_servers` is then ignored."),
			"allow_only_reserved_leases":               tunedBool("Only hand out reserved leases."),
			"block_locally_administered_mac_addresses": tunedBool("Refuse dynamic leases to locally administered (randomised) MAC addresses."),
			"ignore_client_identifier_option":          tunedBool("Identify clients by MAC address instead of option 61."),

			"router_address":         optStr("Default gateway (option 3)."),
			"domain_name":            optStr("Domain name (option 15); enables DNS updates for leases."),
			"domain_search_list":     optList("Domain search list (option 119)."),
			"dns_servers":            optList("DNS servers (option 6)."),
			"wins_servers":           optList("WINS/NBNS servers (option 44)."),
			"ntp_servers":            optList("NTP server addresses (option 42)."),
			"server_address":         optStr("Next server / TFTP address for network boot (siaddr)."),
			"server_host_name":       optStr("Boot server host name (option 66)."),
			"boot_file_name":         optStr("Boot file name (option 67)."),
			"capwap_ac_ip_addresses": optList("CAPWAP access controller addresses (option 138)."),
			"tftp_server_addresses":  optList("TFTP / VoIP configuration server addresses (option 150)."),
			"static_routes":          objList(staticRouteType, "Classless static routes (option 121): objects with `destination`, `subnet_mask`, `router`."),
			"vendor_info":            objList(vendorInfoType, "Vendor-specific information: objects with `identifier` (vendor class match) and `information` (hex)."),
			"generic_options":        objList(genericOptType, "Other DHCP options: objects with `code` and hex `value`."),
			"exclusions":             objList(exclusionType, "Address ranges never handed out dynamically: objects with `starting_address`, `ending_address`."),
		},
	}
}

func (r *dhcpScopeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dhcpScopeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()

	params, diags := scopeParams(ctx, &plan, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.SetScope(ctx, name, params); err != nil {
		resp.Diagnostics.AddError("Error creating DHCP scope", err.Error())
		return
	}
	// New scopes may start enabled, so always set the state explicitly.
	if err := r.client.EnableScope(ctx, name, plan.Enabled.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Error changing DHCP scope state", err.Error())
	}

	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dhcpScopeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dhcpScopeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.GetScope(state.Name.ValueString(), ctx)
	if technitium.IsScopeNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(r.refresh(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dhcpScopeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dhcpScopeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := scopeParams(ctx, &plan, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Name.ValueString() != state.Name.ValueString() {
		params.Set("newName", plan.Name.ValueString())
	}
	if len(params) > 0 {
		if err := r.client.SetScope(ctx, state.Name.ValueString(), params); err != nil {
			resp.Diagnostics.AddError("Error updating DHCP scope", err.Error())
			return
		}
	}
	if !plan.Enabled.Equal(state.Enabled) {
		if err := r.client.EnableScope(ctx, plan.Name.ValueString(), plan.Enabled.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Error changing DHCP scope state", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dhcpScopeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dhcpScopeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteScope(state.Name.ValueString(), ctx); err != nil {
		resp.Diagnostics.AddError("Error deleting DHCP scope", err.Error())
	}
}

func (r *dhcpScopeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func (r *dhcpScopeResource) refresh(ctx context.Context, m *dhcpScopeModel) diag.Diagnostics {
	var diags diag.Diagnostics
	name := m.Name.ValueString()
	s, err := r.client.GetScope(name, ctx)
	if err != nil {
		diags.AddError("Error reading DHCP scope", err.Error())
		return diags
	}
	enabled, _, err := r.client.ScopeEnabled(ctx, name)
	if err != nil {
		diags.AddError("Error reading DHCP scope state", err.Error())
		return diags
	}

	m.ID = types.StringValue(s.Name)
	m.Name = types.StringValue(s.Name)
	m.Enabled = types.BoolValue(enabled)
	m.StartingAddress = types.StringValue(s.StartingAddress)
	m.EndingAddress = types.StringValue(s.EndingAddress)
	m.SubnetMask = types.StringValue(s.SubnetMask)

	m.LeaseTimeDays = types.Int64Value(s.LeaseTimeDays)
	m.LeaseTimeHours = types.Int64Value(s.LeaseTimeHours)
	m.LeaseTimeMinutes = types.Int64Value(s.LeaseTimeMinutes)
	m.OfferDelayTime = types.Int64Value(s.OfferDelayTime)
	m.PingCheckEnabled = types.BoolValue(s.PingCheckEnabled)
	m.PingCheckTimeout = types.Int64Value(s.PingCheckTimeout)
	m.PingCheckRetries = types.Int64Value(s.PingCheckRetries)
	m.DnsUpdates = types.BoolValue(s.DnsUpdates)
	m.DnsOverwriteForDynamicLease = types.BoolValue(s.DnsOverwriteForDynamicLease)
	m.DnsTtl = types.Int64Value(s.DnsTtl)
	m.UseThisDnsServer = types.BoolValue(s.UseThisDnsServer)
	m.AllowOnlyReservedLeases = types.BoolValue(s.AllowOnlyReservedLeases)
	m.BlockLocallyAdministeredMacAddresses = types.BoolValue(s.BlockLocallyAdministeredMacAddresses)
	m.IgnoreClientIdentifierOption = types.BoolValue(s.IgnoreClientIdentifierOption)

	m.RouterAddress = types.StringValue(s.RouterAddress)
	m.DomainName = types.StringValue(s.DomainName)
	m.ServerAddress = types.StringValue(s.ServerAddress)
	m.ServerHostName = types.StringValue(s.ServerHostName)
	m.BootFileName = types.StringValue(s.BootFileName)

	strLists := []struct {
		dst *types.List
		src []string
	}{
		{&m.DomainSearchList, s.DomainSearchList},
		{&m.DnsServers, s.DnsServers},
		{&m.WinsServers, s.WinsServers},
		{&m.NtpServers, s.NtpServers},
		{&m.CapwapAcIpAddresses, s.CapwapAcIpAddresses},
		{&m.TftpServerAddresses, s.TftpServerAddresses},
	}
	for _, l := range strLists {
		v, d := types.ListValueFrom(ctx, types.StringType, nonNil(l.src))
		diags.Append(d...)
		*l.dst = v
	}

	m.Exclusions = objectList(&diags, exclusionType, len(s.Exclusions), func(i int) map[string]attr.Value {
		e := s.Exclusions[i]
		return map[string]attr.Value{"starting_address": types.StringValue(e.StartingAddress), "ending_address": types.StringValue(e.EndingAddress)}
	})
	m.StaticRoutes = objectList(&diags, staticRouteType, len(s.StaticRoutes), func(i int) map[string]attr.Value {
		e := s.StaticRoutes[i]
		return map[string]attr.Value{"destination": types.StringValue(e.Destination), "subnet_mask": types.StringValue(e.SubnetMask), "router": types.StringValue(e.Router)}
	})
	m.VendorInfo = objectList(&diags, vendorInfoType, len(s.VendorInfo), func(i int) map[string]attr.Value {
		e := s.VendorInfo[i]
		return map[string]attr.Value{"identifier": types.StringValue(e.Identifier), "information": types.StringValue(e.Information)}
	})
	m.GenericOptions = objectList(&diags, genericOptType, len(s.GenericOptions), func(i int) map[string]attr.Value {
		e := s.GenericOptions[i]
		return map[string]attr.Value{"code": types.Int64Value(e.Code), "value": types.StringValue(e.Value)}
	})
	return diags
}

func objectList(diags *diag.Diagnostics, t types.ObjectType, n int, item func(int) map[string]attr.Value) types.List {
	elems := make([]attr.Value, 0, n)
	for i := 0; i < n; i++ {
		obj, d := types.ObjectValue(t.AttrTypes, item(i))
		diags.Append(d...)
		elems = append(elems, obj)
	}
	l, d := types.ListValue(t, elems)
	diags.Append(d...)
	return l
}

// scopeParams returns scopes/set params for known plan values that differ
// from state (all known values when state is nil). An empty value clears a
// field on the server.
func scopeParams(ctx context.Context, plan, state *dhcpScopeModel) (url.Values, diag.Diagnostics) {
	var diags diag.Diagnostics
	params := url.Values{}
	type field struct {
		param string
		get   func(*dhcpScopeModel) attr.Value
	}
	fields := []field{
		{"startingAddress", func(m *dhcpScopeModel) attr.Value { return m.StartingAddress }},
		{"endingAddress", func(m *dhcpScopeModel) attr.Value { return m.EndingAddress }},
		{"subnetMask", func(m *dhcpScopeModel) attr.Value { return m.SubnetMask }},
		{"leaseTimeDays", func(m *dhcpScopeModel) attr.Value { return m.LeaseTimeDays }},
		{"leaseTimeHours", func(m *dhcpScopeModel) attr.Value { return m.LeaseTimeHours }},
		{"leaseTimeMinutes", func(m *dhcpScopeModel) attr.Value { return m.LeaseTimeMinutes }},
		{"offerDelayTime", func(m *dhcpScopeModel) attr.Value { return m.OfferDelayTime }},
		{"pingCheckEnabled", func(m *dhcpScopeModel) attr.Value { return m.PingCheckEnabled }},
		{"pingCheckTimeout", func(m *dhcpScopeModel) attr.Value { return m.PingCheckTimeout }},
		{"pingCheckRetries", func(m *dhcpScopeModel) attr.Value { return m.PingCheckRetries }},
		{"dnsUpdates", func(m *dhcpScopeModel) attr.Value { return m.DnsUpdates }},
		{"dnsOverwriteForDynamicLease", func(m *dhcpScopeModel) attr.Value { return m.DnsOverwriteForDynamicLease }},
		{"dnsTtl", func(m *dhcpScopeModel) attr.Value { return m.DnsTtl }},
		{"useThisDnsServer", func(m *dhcpScopeModel) attr.Value { return m.UseThisDnsServer }},
		{"allowOnlyReservedLeases", func(m *dhcpScopeModel) attr.Value { return m.AllowOnlyReservedLeases }},
		{"blockLocallyAdministeredMacAddresses", func(m *dhcpScopeModel) attr.Value { return m.BlockLocallyAdministeredMacAddresses }},
		{"ignoreClientIdentifierOption", func(m *dhcpScopeModel) attr.Value { return m.IgnoreClientIdentifierOption }},
		{"routerAddress", func(m *dhcpScopeModel) attr.Value { return m.RouterAddress }},
		{"domainName", func(m *dhcpScopeModel) attr.Value { return m.DomainName }},
		{"domainSearchList", func(m *dhcpScopeModel) attr.Value { return m.DomainSearchList }},
		{"dnsServers", func(m *dhcpScopeModel) attr.Value { return m.DnsServers }},
		{"winsServers", func(m *dhcpScopeModel) attr.Value { return m.WinsServers }},
		{"ntpServers", func(m *dhcpScopeModel) attr.Value { return m.NtpServers }},
		{"serverAddress", func(m *dhcpScopeModel) attr.Value { return m.ServerAddress }},
		{"serverHostName", func(m *dhcpScopeModel) attr.Value { return m.ServerHostName }},
		{"bootFileName", func(m *dhcpScopeModel) attr.Value { return m.BootFileName }},
		{"capwapAcIpAddresses", func(m *dhcpScopeModel) attr.Value { return m.CapwapAcIpAddresses }},
		{"tftpServerAddresses", func(m *dhcpScopeModel) attr.Value { return m.TftpServerAddresses }},
		{"staticRoutes", func(m *dhcpScopeModel) attr.Value { return m.StaticRoutes }},
		{"vendorInfo", func(m *dhcpScopeModel) attr.Value { return m.VendorInfo }},
		{"genericOptions", func(m *dhcpScopeModel) attr.Value { return m.GenericOptions }},
		{"exclusions", func(m *dhcpScopeModel) attr.Value { return m.Exclusions }},
	}

	for _, f := range fields {
		p := f.get(plan)
		if !known(p) {
			continue
		}
		if state != nil && p.Equal(f.get(state)) {
			continue
		}
		v, d := encodeScopeValue(ctx, f.param, p)
		diags.Append(d...)
		if state == nil && v == "" {
			continue // nothing to clear on a new scope
		}
		params.Set(f.param, v)
	}
	return params, diags
}

func encodeScopeValue(ctx context.Context, param string, v attr.Value) (string, diag.Diagnostics) {
	switch x := v.(type) {
	case types.String:
		return x.ValueString(), nil
	case types.Int64:
		return strconv.FormatInt(x.ValueInt64(), 10), nil
	case types.Bool:
		return strconv.FormatBool(x.ValueBool()), nil
	case types.List:
		var diags diag.Diagnostics
		var parts []string
		switch param {
		case "exclusions":
			var items []struct {
				Start string `tfsdk:"starting_address"`
				End   string `tfsdk:"ending_address"`
			}
			diags = x.ElementsAs(ctx, &items, false)
			for _, it := range items {
				parts = append(parts, it.Start, it.End)
			}
			return strings.Join(parts, "|"), diags
		case "staticRoutes":
			var items []struct {
				Destination string `tfsdk:"destination"`
				SubnetMask  string `tfsdk:"subnet_mask"`
				Router      string `tfsdk:"router"`
			}
			diags = x.ElementsAs(ctx, &items, false)
			for _, it := range items {
				parts = append(parts, it.Destination, it.SubnetMask, it.Router)
			}
			return strings.Join(parts, "|"), diags
		case "vendorInfo":
			var items []struct {
				Identifier  string `tfsdk:"identifier"`
				Information string `tfsdk:"information"`
			}
			diags = x.ElementsAs(ctx, &items, false)
			for _, it := range items {
				parts = append(parts, it.Identifier, it.Information)
			}
			return strings.Join(parts, "|"), diags
		case "genericOptions":
			var items []struct {
				Code  int64  `tfsdk:"code"`
				Value string `tfsdk:"value"`
			}
			diags = x.ElementsAs(ctx, &items, false)
			for _, it := range items {
				parts = append(parts, strconv.FormatInt(it.Code, 10), it.Value)
			}
			return strings.Join(parts, "|"), diags
		default:
			diags = x.ElementsAs(ctx, &parts, false)
			return strings.Join(parts, ","), diags
		}
	}
	return "", nil
}
