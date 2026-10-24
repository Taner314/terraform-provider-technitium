package provider

import (
	"context"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &serverInfoDataSource{}

func NewServerInfoDataSource() datasource.DataSource { return &serverInfoDataSource{} }

type serverInfoDataSource struct {
	client *technitium.Client
}

type serverInfoModel struct {
	Version            types.String `tfsdk:"version"`
	ServerDomain       types.String `tfsdk:"server_domain"`
	StartedAt          types.String `tfsdk:"started_at"`
	ClusterInitialized types.Bool   `tfsdk:"cluster_initialized"`
}

func (d *serverInfoDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_info"
}

func (d *serverInfoDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = ConfigureDataSourceClient(req, resp)
}

func (d *serverInfoDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Read-only information about the Technitium server, e.g. to gate configuration on its version.",
		Attributes: map[string]schema.Attribute{
			"version":             schema.StringAttribute{Computed: true, Description: "Server version, e.g. `15.6.2`."},
			"server_domain":       schema.StringAttribute{Computed: true, Description: "Domain name the server identifies itself with."},
			"started_at":          schema.StringAttribute{Computed: true, Description: "Time the server started (RFC 3339)."},
			"cluster_initialized": schema.BoolAttribute{Computed: true, Description: "Whether clustering is initialized."},
		},
	}
}

func (d *serverInfoDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	s, err := d.client.GetSettings(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading server settings", err.Error())
		return
	}
	str := func(k string) types.String {
		v, _ := s[k].(string)
		return types.StringValue(v)
	}
	cluster, _ := s["clusterInitialized"].(bool)
	resp.Diagnostics.Append(resp.State.Set(ctx, serverInfoModel{
		Version:            str("version"),
		ServerDomain:       str("dnsServerDomain"),
		StartedAt:          str("uptimestamp"),
		ClusterInitialized: types.BoolValue(cluster),
	})...)
}
