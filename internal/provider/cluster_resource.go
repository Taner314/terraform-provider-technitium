package provider

import (
	"context"
	"net/url"
	"strconv"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var clusterNodeType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id": types.Int64Type, "name": types.StringType, "url": types.StringType,
	"ip_addresses": types.ListType{ElemType: types.StringType}, "type": types.StringType, "state": types.StringType,
}}

func clusterNodes(ctx context.Context, diags *diag.Diagnostics, s technitium.ClusterState) types.List {
	return objectList(diags, clusterNodeType, len(s.ClusterNodes), func(i int) map[string]attr.Value {
		n := s.ClusterNodes[i]
		ips, d := types.ListValueFrom(ctx, types.StringType, nonNil(n.IPAddresses))
		diags.Append(d...)
		return map[string]attr.Value{
			"id": types.Int64Value(n.ID), "name": types.StringValue(n.Name), "url": types.StringValue(n.URL),
			"ip_addresses": ips, "type": types.StringValue(n.Type), "state": types.StringValue(n.State),
		}
	})
}

var (
	_ resource.ResourceWithConfigure   = &clusterResource{}
	_ resource.ResourceWithImportState = &clusterResource{}
)

func NewClusterResource() resource.Resource { return &clusterResource{} }

// clusterResource initializes a cluster on the provider's server, which
// becomes the Primary node.
type clusterResource struct {
	client *technitium.Client
}

type clusterModel struct {
	ID                       types.String `tfsdk:"id"`
	ClusterDomain            types.String `tfsdk:"cluster_domain"`
	IPAddresses              types.List   `tfsdk:"ip_addresses"`
	HeartbeatRefreshInterval types.Int64  `tfsdk:"heartbeat_refresh_interval"`
	HeartbeatRetryInterval   types.Int64  `tfsdk:"heartbeat_retry_interval"`
	ConfigRefreshInterval    types.Int64  `tfsdk:"config_refresh_interval"`
	ConfigRetryInterval      types.Int64  `tfsdk:"config_retry_interval"`
	ForceDeleteOnDestroy     types.Bool   `tfsdk:"force_delete_on_destroy"`
	ServerDomain             types.String `tfsdk:"server_domain"`
	Nodes                    types.List   `tfsdk:"nodes"`
}

func (r *clusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *clusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *clusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	interval := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Optional: true, Computed: true, Description: desc,
			PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}
	}
	resp.Schema = schema.Schema{
		Description: "Initializes a cluster with this server as the Primary node. Initializing renames the server to " +
			"`<host>.<cluster_domain>` and enables HTTPS (self-signed if none is configured), and creates the cluster zones; " +
			"don't manage `dns_server_domain` or web service TLS in settings resources at the same time. Destroying deletes the cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Cluster domain.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"cluster_domain": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "Cluster domain name. Cannot be changed after initialization."},
			"ip_addresses": schema.ListAttribute{Required: true, ElementType: types.StringType,
				Description: "This node's addresses reachable by the other nodes."},
			"heartbeat_refresh_interval": interval("Seconds between node state refreshes (10-300)."),
			"heartbeat_retry_interval":   interval("Seconds between retries after a failed refresh (10-300)."),
			"config_refresh_interval":    interval("Seconds between config syncs on Secondary nodes (30-3600)."),
			"config_retry_interval":      interval("Seconds between config sync retries (30-3600)."),
			"force_delete_on_destroy": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "On destroy, delete the cluster even if Secondary nodes remain (orphaning them)."},
			"server_domain": schema.StringAttribute{Computed: true, Description: "This server's name in the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"nodes": schema.ListAttribute{Computed: true, ElementType: clusterNodeType,
				Description: "Cluster nodes: `id`, `name`, `url`, `ip_addresses`, `type` (`Primary`/`Secondary`), `state`."},
		},
	}
}

func (r *clusterResource) options(plan, state *clusterModel) url.Values {
	p := url.Values{}
	for _, f := range []struct {
		key        string
		plan, prev types.Int64
	}{
		{"heartbeatRefreshIntervalSeconds", plan.HeartbeatRefreshInterval, state.HeartbeatRefreshInterval},
		{"heartbeatRetryIntervalSeconds", plan.HeartbeatRetryInterval, state.HeartbeatRetryInterval},
		{"configRefreshIntervalSeconds", plan.ConfigRefreshInterval, state.ConfigRefreshInterval},
		{"configRetryIntervalSeconds", plan.ConfigRetryInterval, state.ConfigRetryInterval},
	} {
		if known(f.plan) && !f.plan.Equal(f.prev) {
			p.Set(f.key, strconv.FormatInt(f.plan.ValueInt64(), 10))
		}
	}
	return p
}

func (r *clusterResource) apply(ctx context.Context, m *clusterModel, s technitium.ClusterState) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(s.ClusterDomain)
	m.ClusterDomain = types.StringValue(s.ClusterDomain)
	m.ServerDomain = types.StringValue(s.DnsServerDomain)
	m.HeartbeatRefreshInterval = types.Int64Value(s.HeartbeatRefreshIntervalSeconds)
	m.HeartbeatRetryInterval = types.Int64Value(s.HeartbeatRetryIntervalSeconds)
	m.ConfigRefreshInterval = types.Int64Value(s.ConfigRefreshIntervalSeconds)
	m.ConfigRetryInterval = types.Int64Value(s.ConfigRetryIntervalSeconds)
	if self, ok := s.Self(); ok {
		ips, d := types.ListValueFrom(ctx, types.StringType, nonNil(self.IPAddresses))
		diags.Append(d...)
		m.IPAddresses = ips
	}
	m.Nodes = clusterNodes(ctx, &diags, s)
	return diags
}

func (r *clusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan clusterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var ips []string
	resp.Diagnostics.Append(plan.IPAddresses.ElementsAs(ctx, &ips, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.InitCluster(ctx, plan.ClusterDomain.ValueString(), ips)
	if err != nil {
		resp.Diagnostics.AddError("Error initializing cluster", err.Error())
		return
	}
	if p := r.options(&plan, &clusterModel{}); len(p) > 0 {
		if s, err = r.client.SetClusterOptions(ctx, p); err != nil {
			resp.Diagnostics.AddError("Error setting cluster options", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state clusterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetClusterState(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster state", err.Error())
		return
	}
	if !s.ClusterInitialized {
		resp.State.RemoveResource(ctx)
		return
	}
	if !known(state.ForceDeleteOnDestroy) {
		state.ForceDeleteOnDestroy = types.BoolValue(false)
	}
	resp.Diagnostics.Append(r.apply(ctx, &state, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *clusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state clusterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.IPAddresses.Equal(state.IPAddresses) {
		var ips []string
		resp.Diagnostics.Append(plan.IPAddresses.ElementsAs(ctx, &ips, false)...)
		if err := r.client.UpdateNodeIPAddresses(ctx, ips); err != nil {
			resp.Diagnostics.AddError("Error updating node addresses", err.Error())
			return
		}
	}
	if p := r.options(&plan, &state); len(p) > 0 {
		if _, err := r.client.SetClusterOptions(ctx, p); err != nil {
			resp.Diagnostics.AddError("Error setting cluster options", err.Error())
			return
		}
	}
	s, err := r.client.GetClusterState(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster state", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *clusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state clusterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteCluster(ctx, state.ForceDeleteOnDestroy.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Error deleting cluster", err.Error())
	}
}

func (r *clusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	s, err := r.client.GetClusterState(ctx)
	if err != nil || !s.ClusterInitialized {
		resp.Diagnostics.AddError("No cluster", "This server is not part of an initialized cluster.")
		return
	}
	m := clusterModel{ForceDeleteOnDestroy: types.BoolValue(false)}
	resp.Diagnostics.Append(r.apply(ctx, &m, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
