package provider

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &dhcpReservedLeaseResource{}
	_ resource.ResourceWithImportState = &dhcpReservedLeaseResource{}
)

func NewDhcpReservedLeaseResource() resource.Resource {
	return &dhcpReservedLeaseResource{}
}

type dhcpReservedLeaseResource struct {
	client *technitium.Client
}

type dhcpReservedLeaseModel struct {
	ID              types.String `tfsdk:"id"`
	Scope           types.String `tfsdk:"scope"`
	HardwareAddress types.String `tfsdk:"hardware_address"`
	IpAddress       types.String `tfsdk:"ip_address"`
	HostName        types.String `tfsdk:"host_name"`
	Comments        types.String `tfsdk:"comments"`
}

// normMAC: the server stores MACs as upper-case, dash-separated.
func normMAC(s string) string {
	return strings.ToUpper(strings.NewReplacer(":", "-", ".", "").Replace(s))
}

func (r *dhcpReservedLeaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *dhcpReservedLeaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dhcp_reserved_lease"
}

func (r *dhcpReservedLeaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Manages a reserved (static) lease in a DHCP scope. Changes to the address, host name or comments " +
			"re-create the reservation, as the API has no update call.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "`<scope>/<hardware_address>`; also the import ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"scope":            schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "DHCP scope name."},
			"hardware_address": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Client MAC address, e.g. `00:11:22:33:44:55`."},
			"ip_address":       schema.StringAttribute{Required: true, Description: "Reserved IP address within the scope's subnet."},
			"host_name": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Description: "Host name to assign, overriding the client's."},
			"comments": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Description: "Comments."},
		},
	}
}

func (r *dhcpReservedLeaseResource) add(ctx context.Context, m dhcpReservedLeaseModel) error {
	return r.client.CreateLease(technitium.DhcpReservedLease{
		Name:            m.Scope.ValueString(),
		HardwareAddress: m.HardwareAddress.ValueString(),
		IpAddress:       m.IpAddress.ValueString(),
		HostName:        m.HostName.ValueString(),
		Comments:        m.Comments.ValueString(),
	}, ctx)
}

// find returns the scope's reservation for m's MAC address.
func (r *dhcpReservedLeaseResource) find(ctx context.Context, m dhcpReservedLeaseModel) (technitium.DhcpReservedLease, bool, error) {
	s, err := r.client.GetScope(m.Scope.ValueString(), ctx)
	if technitium.IsScopeNotFound(err) {
		return technitium.DhcpReservedLease{}, false, nil
	}
	if err != nil {
		return technitium.DhcpReservedLease{}, false, err
	}
	for _, l := range s.ReservedLeases {
		if normMAC(l.HardwareAddress) == normMAC(m.HardwareAddress.ValueString()) {
			return l, true, nil
		}
	}
	return technitium.DhcpReservedLease{}, false, nil
}

func (r *dhcpReservedLeaseResource) apply(m *dhcpReservedLeaseModel, l technitium.DhcpReservedLease) {
	// Keep the configured MAC spelling when it denotes the same address.
	if normMAC(m.HardwareAddress.ValueString()) != normMAC(l.HardwareAddress) {
		m.HardwareAddress = types.StringValue(l.HardwareAddress)
	}
	m.ID = types.StringValue(m.Scope.ValueString() + "/" + m.HardwareAddress.ValueString())
	m.IpAddress = types.StringValue(l.IpAddress)
	m.HostName = types.StringValue(l.HostName)
	m.Comments = types.StringValue(l.Comments)
}

func (r *dhcpReservedLeaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dhcpReservedLeaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.add(ctx, plan); err != nil {
		resp.Diagnostics.AddError("Error creating DHCP reserved lease", err.Error())
		return
	}
	r.readInto(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dhcpReservedLeaseResource) readInto(ctx context.Context, m *dhcpReservedLeaseModel, diags interface{ AddError(string, string) }) {
	l, found, err := r.find(ctx, *m)
	if err != nil {
		diags.AddError("Error reading DHCP reserved lease", err.Error())
		return
	}
	if !found {
		diags.AddError("DHCP reserved lease missing", "The reservation was not found after writing it.")
		return
	}
	r.apply(m, l)
}

func (r *dhcpReservedLeaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dhcpReservedLeaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	l, found, err := r.find(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DHCP reserved lease", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	r.apply(&state, l)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dhcpReservedLeaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dhcpReservedLeaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteLease(state.Scope.ValueString(), state.HardwareAddress.ValueString(), ctx); err != nil {
		resp.Diagnostics.AddError("Error updating DHCP reserved lease", err.Error())
		return
	}
	if err := r.add(ctx, plan); err != nil {
		resp.Diagnostics.AddError("Error updating DHCP reserved lease",
			err.Error()+"\n\nThe previous reservation was removed; re-run apply to recreate it.")
		resp.State.RemoveResource(ctx)
		return
	}
	r.readInto(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dhcpReservedLeaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dhcpReservedLeaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteLease(state.Scope.ValueString(), state.HardwareAddress.ValueString(), ctx); err != nil {
		resp.Diagnostics.AddError("Error deleting DHCP reserved lease", err.Error())
	}
}

func (r *dhcpReservedLeaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	i := strings.LastIndex(req.ID, "/")
	if i <= 0 || i == len(req.ID)-1 {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <scope>/<hardware_address>, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("scope"), req.ID[:i])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("hardware_address"), req.ID[i+1:])...)
}
