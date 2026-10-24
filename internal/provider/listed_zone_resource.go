package provider

import (
	"context"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// listedZoneResource is one entry of the Allowed or Blocked zone list.
type listedZoneResource struct {
	client *technitium.Client
	list   string // "allowed" or "blocked"
}

var (
	_ resource.ResourceWithConfigure   = &listedZoneResource{}
	_ resource.ResourceWithImportState = &listedZoneResource{}
)

func NewAllowedZoneResource() resource.Resource { return &listedZoneResource{list: "allowed"} }
func NewBlockedZoneResource() resource.Resource { return &listedZoneResource{list: "blocked"} }

type listedZoneModel struct {
	ID     types.String `tfsdk:"id"`
	Domain types.String `tfsdk:"domain"`
}

func (r *listedZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.list + "_zone"
}

func (r *listedZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *listedZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	desc := "Blocks a domain and all its subdomains, in addition to block lists from `technitium_settings_blocking`."
	if r.list == "allowed" {
		desc = "Allows a domain and all its subdomains even when a block list or blocked zone matches it."
	}
	resp.Schema = schema.Schema{
		Description: desc,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Domain name; also the import ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"domain": schema.StringAttribute{Required: true, Description: "Domain name, e.g. `ads.example.com`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		},
	}
}

func (r *listedZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan listedZoneModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.AddListedZone(ctx, r.list, plan.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error adding "+r.list+" zone", err.Error())
		return
	}
	plan.ID = types.StringValue(strings.ToLower(strings.TrimSuffix(plan.Domain.ValueString(), ".")))
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *listedZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state listedZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ok, err := r.client.IsListedZone(ctx, r.list, state.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading "+r.list+" zone", err.Error())
		return
	}
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}
	state.ID = types.StringValue(strings.ToLower(strings.TrimSuffix(state.Domain.ValueString(), ".")))
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *listedZoneResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected update", "domain forces replacement.")
}

func (r *listedZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state listedZoneModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteListedZone(ctx, r.list, state.Domain.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting "+r.list+" zone", err.Error())
	}
}

func (r *listedZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), req.ID)...)
}
