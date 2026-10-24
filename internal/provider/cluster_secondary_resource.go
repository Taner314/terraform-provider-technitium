package provider

import (
	"context"
	"strings"
	"time"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithConfigure = &clusterSecondaryResource{}

func NewClusterSecondaryResource() resource.Resource { return &clusterSecondaryResource{} }

// clusterSecondaryResource joins the provider's server to a cluster as a
// Secondary node. Use a provider alias pointing at the secondary.
type clusterSecondaryResource struct {
	client *technitium.Client
}

type clusterSecondaryModel struct {
	ID                      types.String `tfsdk:"id"`
	PrimaryNodeURL          types.String `tfsdk:"primary_node_url"`
	PrimaryNodeIPAddress    types.String `tfsdk:"primary_node_ip_address"`
	PrimaryNodeUsername     types.String `tfsdk:"primary_node_username"`
	PrimaryNodePassword     types.String `tfsdk:"primary_node_password"`
	IgnoreCertificateErrors types.Bool   `tfsdk:"ignore_certificate_errors"`
	IPAddresses             types.List   `tfsdk:"ip_addresses"`
	ForceLeaveOnDestroy     types.Bool   `tfsdk:"force_leave_on_destroy"`
	ClusterDomain           types.String `tfsdk:"cluster_domain"`
	ServerDomain            types.String `tfsdk:"server_domain"`
	Nodes                   types.List   `tfsdk:"nodes"`
}

func (r *clusterSecondaryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster_secondary"
}

func (r *clusterSecondaryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *clusterSecondaryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Joins this server (use a provider alias pointing at it) to a cluster as a Secondary node. " +
			"Joining overwrites this server's Allowed, Blocked, Apps, Settings and Administration config with the Primary's. " +
			"Destroying leaves the cluster.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, Description: "This node's name in the cluster.", PlanModifiers: keep},
			"primary_node_url": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "HTTPS web service URL of the Primary, e.g. `https://dns1.example.com:53443/`."},
			"primary_node_ip_address": schema.StringAttribute{Optional: true, PlanModifiers: replace,
				Description: "Primary's IP address, if its name in the URL doesn't resolve from here."},
			"primary_node_username": schema.StringAttribute{Required: true, Description: "Administrator on the Primary; only used to join."},
			"primary_node_password": schema.StringAttribute{Required: true, Sensitive: true, Description: "Password of that administrator; only used to join."},
			"ignore_certificate_errors": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:   "Accept the Primary's self-signed certificate (private networks only)."},
			"ip_addresses": schema.ListAttribute{Required: true, ElementType: types.StringType,
				Description: "This node's addresses reachable by the other nodes."},
			"force_leave_on_destroy": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "On destroy, leave without notifying the Primary (when it is gone)."},
			"cluster_domain": schema.StringAttribute{Computed: true, Description: "Cluster domain.", PlanModifiers: keep},
			"server_domain":  schema.StringAttribute{Computed: true, Description: "This server's name in the cluster.", PlanModifiers: keep},
			"nodes": schema.ListAttribute{Computed: true, ElementType: clusterNodeType, Description: "Cluster nodes as seen from this node.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *clusterSecondaryResource) apply(ctx context.Context, m *clusterSecondaryModel, s technitium.ClusterState) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(s.DnsServerDomain)
	m.ClusterDomain = types.StringValue(s.ClusterDomain)
	m.ServerDomain = types.StringValue(s.DnsServerDomain)
	if self, ok := s.Self(); ok {
		m.IPAddresses, _ = types.ListValueFrom(ctx, types.StringType, nonNil(self.IPAddresses))
	}
	m.Nodes = clusterNodes(ctx, &diags, s)
	return diags
}

func (r *clusterSecondaryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan clusterSecondaryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var ips []string
	resp.Diagnostics.Append(plan.IPAddresses.ElementsAs(ctx, &ips, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	join := technitium.JoinCluster{
		SecondaryNodeIPAddresses: ips,
		PrimaryNodeURL:           plan.PrimaryNodeURL.ValueString(),
		PrimaryNodeIPAddress:     plan.PrimaryNodeIPAddress.ValueString(),
		IgnoreCertificateErrors:  plan.IgnoreCertificateErrors.ValueBool(),
		PrimaryNodeUsername:      plan.PrimaryNodeUsername.ValueString(),
		PrimaryNodePassword:      plan.PrimaryNodePassword.ValueString(),
	}
	// A freshly initialized Primary restarts its web service to enable
	// HTTPS, so retry while it isn't reachable yet.
	var s technitium.ClusterState
	var err error
	for deadline := time.Now().Add(2 * time.Minute); ; {
		s, err = r.client.JoinCluster(ctx, join)
		if err == nil || time.Now().After(deadline) || !isUnreachable(err) {
			break
		}
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Error joining cluster", ctx.Err().Error())
			return
		case <-time.After(5 * time.Second):
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Error joining cluster", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *clusterSecondaryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state clusterSecondaryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetClusterState(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster state", err.Error())
		return
	}
	self, ok := s.Self()
	if !s.ClusterInitialized || !ok || self.Type != "Secondary" {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &state, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *clusterSecondaryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state clusterSecondaryModel
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
	s, err := r.client.GetClusterState(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading cluster state", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan, s)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *clusterSecondaryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state clusterSecondaryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.LeaveCluster(ctx, state.ForceLeaveOnDestroy.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Error leaving cluster", err.Error())
	}
}

func isUnreachable(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{"connection refused", "timed out", "no route to host", "actively refused"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}
