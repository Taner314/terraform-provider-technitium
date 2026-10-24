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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &dnsZoneResource{}
	_ resource.ResourceWithImportState = &dnsZoneResource{}
)

func NewDnsZoneResource() resource.Resource {
	return &dnsZoneResource{}
}

type dnsZoneResource struct {
	client *technitium.Client
}

type dnsZoneResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Type types.String `tfsdk:"type"`

	// Create-only.
	UseSoaSerialDateScheme types.Bool   `tfsdk:"use_soa_serial_date_scheme"`
	Forwarder              types.String `tfsdk:"forwarder"`
	ForwarderProtocol      types.String `tfsdk:"forwarder_protocol"`
	DnssecValidation       types.Bool   `tfsdk:"dnssec_validation"`

	// Zone options, managed in place.
	Disabled                           types.Bool   `tfsdk:"disabled"`
	Catalog                            types.String `tfsdk:"catalog"`
	OverrideCatalogQueryAccess         types.Bool   `tfsdk:"override_catalog_query_access"`
	OverrideCatalogZoneTransfer        types.Bool   `tfsdk:"override_catalog_zone_transfer"`
	OverrideCatalogNotify              types.Bool   `tfsdk:"override_catalog_notify"`
	PrimaryNameServerAddresses         types.List   `tfsdk:"primary_name_server_addresses"`
	PrimaryZoneTransferProtocol        types.String `tfsdk:"primary_zone_transfer_protocol"`
	PrimaryZoneTransferTsigKeyName     types.String `tfsdk:"primary_zone_transfer_tsig_key_name"`
	ValidateZone                       types.Bool   `tfsdk:"validate_zone"`
	QueryAccess                        types.String `tfsdk:"query_access"`
	QueryAccessNetworkACL              types.List   `tfsdk:"query_access_network_acl"`
	ZoneTransfer                       types.String `tfsdk:"zone_transfer"`
	ZoneTransferNetworkACL             types.List   `tfsdk:"zone_transfer_network_acl"`
	ZoneTransferTsigKeyNames           types.List   `tfsdk:"zone_transfer_tsig_key_names"`
	Notify                             types.String `tfsdk:"notify"`
	NotifyNameServers                  types.List   `tfsdk:"notify_name_servers"`
	NotifySecondaryCatalogsNameServers types.List   `tfsdk:"notify_secondary_catalogs_name_servers"`
	Update                             types.String `tfsdk:"update"`
	UpdateNetworkACL                   types.List   `tfsdk:"update_network_acl"`
	UpdateSecurityPolicies             types.List   `tfsdk:"update_security_policies"`

	// Computed.
	DnssecStatus types.String `tfsdk:"dnssec_status"`
	Internal     types.Bool   `tfsdk:"internal"`
}

var securityPolicyType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"tsig_key_name": types.StringType,
	"domain":        types.StringType,
	"allowed_types": types.ListType{ElemType: types.StringType},
}}

func (r *dnsZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *dnsZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (r *dnsZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keepStr := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	keepBool := []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}
	keepList := []planmodifier.List{listplanmodifier.UseStateForUnknown()}

	optStr := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Description: desc, PlanModifiers: keepStr}
	}
	optBool := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Optional: true, Computed: true, Description: desc, PlanModifiers: keepBool}
	}
	optList := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: desc, PlanModifiers: keepList}
	}

	resp.Schema = schema.Schema{
		Description: "Manages an authoritative zone and its options. Options left unset follow the server's current value; " +
			"only the options valid for the zone `type` should be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Zone name.", PlanModifiers: keepStr},
			"name": schema.StringAttribute{
				Required: true, PlanModifiers: replace,
				Description: "Zone name. For reverse zones use the `in-addr.arpa`/`ip6.arpa` name.",
			},
			"type": schema.StringAttribute{
				Required: true, PlanModifiers: replace,
				Description: "`Primary`, `Secondary`, `Stub`, `Forwarder`, `SecondaryForwarder`, `Catalog` or `SecondaryCatalog`.",
			},

			"use_soa_serial_date_scheme": schema.BoolAttribute{
				Optional: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description: "Use the YYYYMMDDnn SOA serial scheme (`Primary`, `Forwarder`, `Catalog`). Applied at creation.",
			},
			"forwarder": schema.StringAttribute{
				Optional: true, PlanModifiers: replace,
				Description: "`Forwarder` zones: create the zone with this initial FWD record. Prefer `technitium_dns_record_fwd` to manage forwarders.",
			},
			"forwarder_protocol": schema.StringAttribute{
				Optional: true, PlanModifiers: replace,
				Description: "Protocol of the initial FWD record: `Udp`, `Tcp`, `Tls`, `Https` or `Quic`.",
			},
			"dnssec_validation": schema.BoolAttribute{
				Optional: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description: "DNSSEC validation for the initial FWD record.",
			},

			"disabled":                               optBool("Whether the zone is disabled."),
			"catalog":                                optStr("Catalog zone this zone is a member of; empty string for none."),
			"override_catalog_query_access":          optBool("Override the catalog zone's query access setting."),
			"override_catalog_zone_transfer":         optBool("Override the catalog zone's zone transfer setting."),
			"override_catalog_notify":                optBool("Override the catalog zone's notify setting."),
			"primary_name_server_addresses":          optList("`Secondary`/`Stub` zones: primary name server addresses."),
			"primary_zone_transfer_protocol":         optStr("`Secondary` zones: `Tcp`, `Tls` or `Quic`."),
			"primary_zone_transfer_tsig_key_name":    optStr("`Secondary` zones: TSIG key used for zone transfer."),
			"validate_zone":                          optBool("`Secondary` zones: validate with ZONEMD after each transfer."),
			"query_access":                           optStr("`Deny`, `Allow`, `AllowOnlyPrivateNetworks`, `AllowOnlyZoneNameServers`, `UseSpecifiedNetworkACL` or `AllowZoneNameServersAndUseSpecifiedNetworkACL`."),
			"query_access_network_acl":               optList("Ordered ACL for query access; prefix an entry with `!` to deny."),
			"zone_transfer":                          optStr("`Deny`, `Allow`, `AllowOnlyZoneNameServers`, `UseSpecifiedNetworkACL` or `AllowZoneNameServersAndUseSpecifiedNetworkACL`."),
			"zone_transfer_network_acl":              optList("Ordered ACL for zone transfer."),
			"zone_transfer_tsig_key_names":           optList("TSIG key names allowed to transfer the zone."),
			"notify":                                 optStr("`None`, `ZoneNameServers`, `SpecifiedNameServers`, `BothZoneAndSpecifiedNameServers` or `SeparateNameServersForCatalogAndMemberZones`."),
			"notify_name_servers":                    optList("Name server addresses to notify."),
			"notify_secondary_catalogs_name_servers": optList("`Catalog` zones: addresses notified only for catalog updates."),
			"update":                                 optStr("Dynamic updates (RFC 2136): `Deny`, `Allow`, `AllowOnlyZoneNameServers`, `UseSpecifiedNetworkACL` or `AllowZoneNameServersAndUseSpecifiedNetworkACL`."),
			"update_network_acl":                     optList("Ordered ACL for dynamic updates."),
			"update_security_policies": schema.ListAttribute{
				Optional: true, Computed: true, ElementType: securityPolicyType, PlanModifiers: keepList,
				Description: "Dynamic update policies: objects with `tsig_key_name`, `domain` (wildcards allowed) and `allowed_types`.",
			},

			"dnssec_status": schema.StringAttribute{Computed: true, Description: "DNSSEC status, e.g. `Unsigned`.", PlanModifiers: keepStr},
			"internal":      schema.BoolAttribute{Computed: true, Description: "Whether the zone is a server-internal zone.", PlanModifiers: keepBool},
		},
	}
}

func (r *dnsZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	create := technitium.DnsZoneCreate{
		Name:                   plan.Name.ValueString(),
		Type:                   plan.Type.ValueString(),
		Catalog:                plan.Catalog.ValueString(),
		UseSoaSerialDateScheme: plan.UseSoaSerialDateScheme.ValueBool(),
		ZoneTransferProtocol:   plan.PrimaryZoneTransferProtocol.ValueString(),
		TsigKeyName:            plan.PrimaryZoneTransferTsigKeyName.ValueString(),
		ValidateZone:           plan.ValidateZone.ValueBool(),
		Forwarder:              plan.Forwarder.ValueString(),
		InitializeForwarder:    plan.Forwarder.ValueString() != "",
		Protocol:               plan.ForwarderProtocol.ValueString(),
		DnssecValidation:       plan.DnssecValidation.ValueBool(),
	}
	if known(plan.PrimaryNameServerAddresses) {
		resp.Diagnostics.Append(plan.PrimaryNameServerAddresses.ElementsAs(ctx, &create.PrimaryNameServerAddresses, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.CreateDnsZone(create, ctx); err != nil {
		resp.Diagnostics.AddError("Error creating zone", err.Error())
		return
	}

	// Create-time params already covered these.
	skip := map[string]bool{"catalog": true, "primaryNameServerAddresses": true, "primaryZoneTransferProtocol": true,
		"primaryZoneTransferTsigKeyName": true, "validateZone": true}
	params, diags := zoneOptionParams(ctx, nil, &plan, skip)
	resp.Diagnostics.Append(diags...)
	if len(params) > 0 && !resp.Diagnostics.HasError() {
		if err := r.client.SetDnsZoneOptions(ctx, plan.Name.ValueString(), params); err != nil {
			resp.Diagnostics.AddError("Error setting zone options", err.Error()+
				"\n\nThe zone was created; it is saved to state so it can be fixed or destroyed.")
		}
	}

	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dnsZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.GetDnsZone(state.Name.ValueString(), ctx)
	if technitium.IsZoneNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading zone", err.Error())
		return
	}

	resp.Diagnostics.Append(applyZone(ctx, &state, zone)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dnsZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dnsZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := zoneOptionParams(ctx, &state, &plan, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(params) > 0 {
		if err := r.client.SetDnsZoneOptions(ctx, plan.Name.ValueString(), params); err != nil {
			resp.Diagnostics.AddError("Error updating zone options", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dnsZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteDnsZone(state.Name.ValueString(), ctx); err != nil {
		resp.Diagnostics.AddError("Error deleting zone", err.Error())
	}
}

func (r *dnsZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), strings.TrimSuffix(req.ID, "."))...)
	// Read fills the rest; null lists/strings are replaced by server values.
	zone, err := r.client.GetDnsZone(strings.TrimSuffix(req.ID, "."), ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error importing zone", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), zone.Type)...)
}

func (r *dnsZoneResource) refresh(ctx context.Context, m *dnsZoneResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	zone, err := r.client.GetDnsZone(m.Name.ValueString(), ctx)
	if err != nil {
		diags.AddError("Error reading zone after write", err.Error())
		return diags
	}
	diags.Append(applyZone(ctx, m, zone)...)
	return diags
}

// applyZone copies server options into m. Options the zone type doesn't have
// become null.
func applyZone(ctx context.Context, m *dnsZoneResourceModel, z technitium.DnsZone) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(z.Name)
	m.Type = types.StringValue(z.Type)
	m.Disabled = types.BoolValue(z.Disabled)
	m.DnssecStatus = types.StringValue(z.DnssecStatus)
	m.Internal = types.BoolValue(z.Internal)

	m.Catalog = optString(z.Catalog, isCatalogMemberType(z.Type))
	// Only returned while the zone is a catalog member; false otherwise.
	member := isCatalogMemberType(z.Type)
	m.OverrideCatalogQueryAccess = optBool(z.OverrideCatalogQueryAccess, member)
	m.OverrideCatalogZoneTransfer = optBool(z.OverrideCatalogZoneTransfer, member)
	m.OverrideCatalogNotify = optBool(z.OverrideCatalogNotify, member)
	m.PrimaryZoneTransferProtocol = types.StringPointerValue(z.PrimaryZoneTransferProtocol)
	m.PrimaryZoneTransferTsigKeyName = types.StringPointerValue(z.PrimaryZoneTransferTsigKeyName)
	m.ValidateZone = types.BoolPointerValue(z.ValidateZone)
	m.QueryAccess = types.StringPointerValue(z.QueryAccess)
	m.ZoneTransfer = types.StringPointerValue(z.ZoneTransfer)
	m.Notify = types.StringPointerValue(z.Notify)
	m.Update = types.StringPointerValue(z.Update)

	lists := []struct {
		dst     *types.List
		src     []string
		present bool
	}{
		{&m.PrimaryNameServerAddresses, z.PrimaryNameServerAddresses, z.PrimaryNameServerAddresses != nil},
		{&m.QueryAccessNetworkACL, z.QueryAccessNetworkACL, z.QueryAccess != nil},
		{&m.ZoneTransferNetworkACL, z.ZoneTransferNetworkACL, z.ZoneTransfer != nil},
		{&m.ZoneTransferTsigKeyNames, z.ZoneTransferTsigKeyNames, z.ZoneTransferTsigKeyNames != nil},
		{&m.NotifyNameServers, z.NotifyNameServers, z.Notify != nil},
		{&m.NotifySecondaryCatalogsNameServers, z.NotifySecondaryCatalogsNameServers, z.NotifySecondaryCatalogsNameServers != nil},
		{&m.UpdateNetworkACL, z.UpdateNetworkACL, z.Update != nil},
	}
	for _, l := range lists {
		if !l.present {
			*l.dst = types.ListNull(types.StringType)
			continue
		}
		v, d := types.ListValueFrom(ctx, types.StringType, nonNil(l.src))
		diags.Append(d...)
		*l.dst = v
	}

	if z.Update == nil {
		m.UpdateSecurityPolicies = types.ListNull(securityPolicyType)
	} else {
		policies := make([]attr.Value, 0, len(z.UpdateSecurityPolicies))
		for _, p := range z.UpdateSecurityPolicies {
			allowed, d := types.ListValueFrom(ctx, types.StringType, nonNil(p.AllowedTypes))
			diags.Append(d...)
			obj, d := types.ObjectValue(securityPolicyType.AttrTypes, map[string]attr.Value{
				"tsig_key_name": types.StringValue(p.TsigKeyName),
				"domain":        types.StringValue(p.Domain),
				"allowed_types": allowed,
			})
			diags.Append(d...)
			policies = append(policies, obj)
		}
		v, d := types.ListValue(securityPolicyType, policies)
		diags.Append(d...)
		m.UpdateSecurityPolicies = v
	}
	return diags
}

func isCatalogMemberType(t string) bool {
	switch t {
	case "Primary", "Secondary", "Stub", "Forwarder":
		return true
	}
	return false
}

// optString maps the API's null catalog to "" for zone types that can have
// one, so "no catalog" is a plain empty string in config.
func optString(p *string, emptyIfNil bool) types.String {
	if p == nil {
		if emptyIfNil {
			return types.StringValue("")
		}
		return types.StringNull()
	}
	return types.StringValue(*p)
}

func optBool(p *bool, falseIfNil bool) types.Bool {
	if p == nil && falseIfNil {
		return types.BoolValue(false)
	}
	return types.BoolPointerValue(p)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// zoneOptionParams returns options/set params for plan values that are known
// and differ from state (state nil = creating).
func zoneOptionParams(ctx context.Context, state, plan *dnsZoneResourceModel, skip map[string]bool) (url.Values, diag.Diagnostics) {
	var diags diag.Diagnostics
	params := url.Values{}
	changed := func(p, s attr.Value) bool {
		return known(p) && (s == nil || !p.Equal(s))
	}
	pick := func(get func(*dnsZoneResourceModel) attr.Value) (attr.Value, attr.Value) {
		if state == nil {
			return get(plan), nil
		}
		return get(plan), get(state)
	}

	type opt struct {
		param string
		get   func(*dnsZoneResourceModel) attr.Value
	}
	opts := []opt{
		{"disabled", func(m *dnsZoneResourceModel) attr.Value { return m.Disabled }},
		{"catalog", func(m *dnsZoneResourceModel) attr.Value { return m.Catalog }},
		{"overrideCatalogQueryAccess", func(m *dnsZoneResourceModel) attr.Value { return m.OverrideCatalogQueryAccess }},
		{"overrideCatalogZoneTransfer", func(m *dnsZoneResourceModel) attr.Value { return m.OverrideCatalogZoneTransfer }},
		{"overrideCatalogNotify", func(m *dnsZoneResourceModel) attr.Value { return m.OverrideCatalogNotify }},
		{"primaryNameServerAddresses", func(m *dnsZoneResourceModel) attr.Value { return m.PrimaryNameServerAddresses }},
		{"primaryZoneTransferProtocol", func(m *dnsZoneResourceModel) attr.Value { return m.PrimaryZoneTransferProtocol }},
		{"primaryZoneTransferTsigKeyName", func(m *dnsZoneResourceModel) attr.Value { return m.PrimaryZoneTransferTsigKeyName }},
		{"validateZone", func(m *dnsZoneResourceModel) attr.Value { return m.ValidateZone }},
		{"queryAccess", func(m *dnsZoneResourceModel) attr.Value { return m.QueryAccess }},
		{"queryAccessNetworkACL", func(m *dnsZoneResourceModel) attr.Value { return m.QueryAccessNetworkACL }},
		{"zoneTransfer", func(m *dnsZoneResourceModel) attr.Value { return m.ZoneTransfer }},
		{"zoneTransferNetworkACL", func(m *dnsZoneResourceModel) attr.Value { return m.ZoneTransferNetworkACL }},
		{"zoneTransferTsigKeyNames", func(m *dnsZoneResourceModel) attr.Value { return m.ZoneTransferTsigKeyNames }},
		{"notify", func(m *dnsZoneResourceModel) attr.Value { return m.Notify }},
		{"notifyNameServers", func(m *dnsZoneResourceModel) attr.Value { return m.NotifyNameServers }},
		{"notifySecondaryCatalogsNameServers", func(m *dnsZoneResourceModel) attr.Value { return m.NotifySecondaryCatalogsNameServers }},
		{"update", func(m *dnsZoneResourceModel) attr.Value { return m.Update }},
		{"updateNetworkACL", func(m *dnsZoneResourceModel) attr.Value { return m.UpdateNetworkACL }},
		{"updateSecurityPolicies", func(m *dnsZoneResourceModel) attr.Value { return m.UpdateSecurityPolicies }},
	}

	for _, o := range opts {
		if skip[o.param] {
			continue
		}
		p, s := pick(o.get)
		if !changed(p, s) {
			continue
		}
		switch v := p.(type) {
		case types.String:
			// Sent even when empty: "catalog=" removes catalog membership.
			params.Set(o.param, v.ValueString())
		case types.Bool:
			params.Set(o.param, strconv.FormatBool(v.ValueBool()))
		case types.List:
			if o.param == "updateSecurityPolicies" {
				params.Set(o.param, encodePolicies(ctx, v, &diags))
				continue
			}
			var items []string
			diags.Append(v.ElementsAs(ctx, &items, false)...)
			if len(items) == 0 {
				params.Set(o.param, "false") // clears the list
			} else {
				params.Set(o.param, strings.Join(items, ","))
			}
		}
	}
	return params, diags
}

func encodePolicies(ctx context.Context, l types.List, diags *diag.Diagnostics) string {
	var policies []struct {
		TsigKeyName  string   `tfsdk:"tsig_key_name"`
		Domain       string   `tfsdk:"domain"`
		AllowedTypes []string `tfsdk:"allowed_types"`
	}
	diags.Append(l.ElementsAs(ctx, &policies, false)...)
	if len(policies) == 0 {
		return "false"
	}
	parts := make([]string, 0, len(policies)*3)
	for _, p := range policies {
		parts = append(parts, p.TsigKeyName, p.Domain, strings.Join(p.AllowedTypes, ","))
	}
	return strings.Join(parts, "|")
}
