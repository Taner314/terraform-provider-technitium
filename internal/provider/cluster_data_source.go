package provider

import (
	"context"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &clusterDataSource{}

func NewClusterDataSource() datasource.DataSource { return &clusterDataSource{} }

// clusterDataSource reports cluster state. Clusters are set up in the web
// console; managing them from Terraform conflicts with cluster config sync.
type clusterDataSource struct {
	client *technitium.Client
}

var clusterNodeType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id": types.Int64Type, "name": types.StringType, "url": types.StringType,
	"ip_addresses": types.ListType{ElemType: types.StringType}, "type": types.StringType,
	"state": types.StringType, "version": types.StringType,
}}

type clusterDataModel struct {
	Initialized              types.Bool   `tfsdk:"initialized"`
	ClusterDomain            types.String `tfsdk:"cluster_domain"`
	ServerDomain             types.String `tfsdk:"server_domain"`
	NodeType                 types.String `tfsdk:"node_type"`
	HeartbeatRefreshInterval types.Int64  `tfsdk:"heartbeat_refresh_interval"`
	HeartbeatRetryInterval   types.Int64  `tfsdk:"heartbeat_retry_interval"`
	ConfigRefreshInterval    types.Int64  `tfsdk:"config_refresh_interval"`
	ConfigRetryInterval      types.Int64  `tfsdk:"config_retry_interval"`
	Nodes                    types.List   `tfsdk:"nodes"`
}

func (d *clusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (d *clusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = ConfigureDataSourceClient(req, resp)
}

func (d *clusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the cluster state of the server. Clusters are set up in the web console; " +
			"on a Secondary node most configuration is synced from the Primary.",
		Attributes: map[string]schema.Attribute{
			"initialized":                schema.BoolAttribute{Computed: true, Description: "Whether the server is part of a cluster."},
			"cluster_domain":             schema.StringAttribute{Computed: true, Description: "Cluster domain; empty when not clustered."},
			"server_domain":              schema.StringAttribute{Computed: true, Description: "This server's name."},
			"node_type":                  schema.StringAttribute{Computed: true, Description: "This node's role: `Primary` or `Secondary`; empty when not clustered."},
			"heartbeat_refresh_interval": schema.Int64Attribute{Computed: true, Description: "Seconds between node state refreshes."},
			"heartbeat_retry_interval":   schema.Int64Attribute{Computed: true, Description: "Seconds between retries after a failed refresh."},
			"config_refresh_interval":    schema.Int64Attribute{Computed: true, Description: "Seconds between config syncs on Secondary nodes."},
			"config_retry_interval":      schema.Int64Attribute{Computed: true, Description: "Seconds between config sync retries."},
			"nodes": schema.ListAttribute{Computed: true, ElementType: clusterNodeType,
				Description: "Nodes: `id`, `name`, `url`, `ip_addresses`, `type`, `state` (`Self`, `Connected`, `Unreachable`, ...), `version`."},
		},
	}
}

func (d *clusterDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	s, err := d.client.GetClusterState(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster state", err.Error())
		return
	}
	var diags diag.Diagnostics
	nodes := objectList(&diags, clusterNodeType, len(s.ClusterNodes), func(i int) map[string]attr.Value {
		n := s.ClusterNodes[i]
		ips, dd := types.ListValueFrom(ctx, types.StringType, nonNil(n.IPAddresses))
		diags.Append(dd...)
		return map[string]attr.Value{
			"id": types.Int64Value(n.ID), "name": types.StringValue(n.Name), "url": types.StringValue(n.URL),
			"ip_addresses": ips, "type": types.StringValue(n.Type), "state": types.StringValue(n.State),
			"version": types.StringValue(n.Version),
		}
	})
	resp.Diagnostics.Append(diags...)
	nodeType := ""
	if self, ok := s.Self(); ok {
		nodeType = self.Type
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, clusterDataModel{
		Initialized:              types.BoolValue(s.ClusterInitialized),
		ClusterDomain:            types.StringValue(s.ClusterDomain),
		ServerDomain:             types.StringValue(s.DnsServerDomain),
		NodeType:                 types.StringValue(nodeType),
		HeartbeatRefreshInterval: types.Int64Value(s.HeartbeatRefreshIntervalSeconds),
		HeartbeatRetryInterval:   types.Int64Value(s.HeartbeatRetryIntervalSeconds),
		ConfigRefreshInterval:    types.Int64Value(s.ConfigRefreshIntervalSeconds),
		ConfigRetryInterval:      types.Int64Value(s.ConfigRetryIntervalSeconds),
		Nodes:                    nodes,
	})...)
}
