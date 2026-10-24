package provider

import (
	"context"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &dhcpLeasesDataSource{}

func NewDhcpLeasesDataSource() datasource.DataSource { return &dhcpLeasesDataSource{} }

type dhcpLeasesDataSource struct {
	client *technitium.Client
}

var dhcpLeaseType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"scope": types.StringType, "type": types.StringType, "hardware_address": types.StringType,
	"client_identifier": types.StringType, "address": types.StringType, "host_name": types.StringType,
	"lease_obtained": types.StringType, "lease_expires": types.StringType,
}}

func (d *dhcpLeasesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_leases"
}

func (d *dhcpLeasesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = ConfigureDataSourceClient(req, resp)
}

func (d *dhcpLeasesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists DHCP leases currently held by clients, dynamic or reserved. A reservation shows up once a client claims it; " +
			"unclaimed reservations are only part of the scope.",
		Attributes: map[string]schema.Attribute{
			"scope": schema.StringAttribute{Optional: true, Description: "Only leases from this scope."},
			"leases": schema.ListAttribute{Computed: true, ElementType: dhcpLeaseType,
				Description: "Leases: `scope`, `type` (`Dynamic`/`Reserved`), `hardware_address`, `client_identifier`, `address`, `host_name`, `lease_obtained`, `lease_expires`."},
		},
	}
}

func (d *dhcpLeasesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var scope types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRoot("scope"), &scope)...)
	leases, err := d.client.ListLeases(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing DHCP leases", err.Error())
		return
	}
	elems := []attr.Value{}
	for _, l := range leases {
		if known(scope) && scope.ValueString() != "" && l.Scope != scope.ValueString() {
			continue
		}
		obj, diags := types.ObjectValue(dhcpLeaseType.AttrTypes, map[string]attr.Value{
			"scope": types.StringValue(l.Scope), "type": types.StringValue(l.Type),
			"hardware_address": types.StringValue(l.HardwareAddress), "client_identifier": types.StringValue(l.ClientIdentifier),
			"address": types.StringValue(l.Address), "host_name": types.StringValue(l.HostName),
			"lease_obtained": types.StringValue(l.LeaseObtained), "lease_expires": types.StringValue(l.LeaseExpires),
		})
		resp.Diagnostics.Append(diags...)
		elems = append(elems, obj)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathRoot("scope"), scope)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, pathRoot("leases"), types.ListValueMust(dhcpLeaseType, elems))...)
}
