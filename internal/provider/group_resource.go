package provider

import (
	"context"
	"net/url"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &groupResource{}
	_ resource.ResourceWithImportState = &groupResource{}
)

func NewGroupResource() resource.Resource { return &groupResource{} }

type groupResource struct {
	client *technitium.Client
}

type groupModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Members     types.Set    `tfsdk:"members"`
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a user group. Assign users with `technitium_user.groups`; grant access with `technitium_permission`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Group name.", PlanModifiers: []planmodifier.String{copyFrom("name")}},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Group name. Changing it renames the group in place.",
			},
			"description": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString(""),
				Description: "Group description.",
			},
			"members": schema.SetAttribute{
				Computed: true, ElementType: types.StringType,
				Description:   "Usernames in the group (read-only).",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.CreateGroup(ctx, plan.Name.ValueString(), plan.Description.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error creating group", err.Error())
		return
	}
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.GetGroup(ctx, state.Name.ValueString())
	if technitium.IsGroupNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading group", err.Error())
		return
	}
	resp.Diagnostics.Append(applyGroup(ctx, &state, g)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		params.Set("newGroup", plan.Name.ValueString())
	}
	params.Set("description", plan.Description.ValueString())
	if _, err := r.client.SetGroup(ctx, state.Name.ValueString(), params); err != nil {
		resp.Diagnostics.AddError("Error updating group", err.Error())
		return
	}
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteGroup(ctx, state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting group", err.Error())
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func (r *groupResource) refresh(ctx context.Context, m *groupModel) diag.Diagnostics {
	var diags diag.Diagnostics
	g, err := r.client.GetGroup(ctx, m.Name.ValueString())
	if err != nil {
		diags.AddError("Error reading group", err.Error())
		return diags
	}
	return applyGroup(ctx, m, g)
}

func applyGroup(ctx context.Context, m *groupModel, g technitium.Group) diag.Diagnostics {
	m.ID = types.StringValue(g.Name)
	m.Name = types.StringValue(g.Name)
	m.Description = types.StringValue(g.Description)
	members, diags := types.SetValueFrom(ctx, types.StringType, nonNil(g.Members))
	m.Members = members
	return diags
}
