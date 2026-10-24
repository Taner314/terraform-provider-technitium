package provider

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
)

func NewUserResource() resource.Resource { return &userResource{} }

type userResource struct {
	client *technitium.Client
}

type userModel struct {
	ID                    types.String `tfsdk:"id"`
	Username              types.String `tfsdk:"username"`
	Password              types.String `tfsdk:"password"`
	DisplayName           types.String `tfsdk:"display_name"`
	Disabled              types.Bool   `tfsdk:"disabled"`
	SessionTimeoutSeconds types.Int64  `tfsdk:"session_timeout_seconds"`
	Groups                types.Set    `tfsdk:"groups"`
	TotpEnabled           types.Bool   `tfsdk:"totp_enabled"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a local user account. Group membership is managed here, not on `technitium_group`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Username.", PlanModifiers: []planmodifier.String{copyFrom("username")}},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username. Changing it renames the account in place.",
			},
			"password": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				Description: "Password. The server cannot return it, so changes made outside Terraform are not detected; " +
					"after an import the first apply resets it to this value.",
			},
			"display_name": schema.StringAttribute{
				Optional: true, Computed: true,
				Description:   "Display name. Defaults to the username.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"disabled": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Disable the account; this also ends its sessions.",
			},
			"session_timeout_seconds": schema.Int64Attribute{
				Optional: true, Computed: true,
				Description:   "Session timeout in seconds. Defaults to the server's value (1800).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"groups": schema.SetAttribute{
				Optional: true, Computed: true, ElementType: types.StringType,
				Description:   "Groups the user belongs to. `Everyone` is implicit and cannot be listed.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
				Validators: []validator.Set{setvalidator.ValueStringsAre(
					stringvalidator.NoneOfCaseInsensitive("Everyone"),
				)},
			},
			"totp_enabled": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the user has enabled two-factor authentication.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	username := plan.Username.ValueString()
	if _, err := r.client.CreateUser(ctx, username, plan.Password.ValueString(), plan.DisplayName.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error creating user", err.Error())
		return
	}

	params := url.Values{}
	if plan.Disabled.ValueBool() {
		params.Set("disabled", "true")
	}
	if known(plan.SessionTimeoutSeconds) {
		params.Set("sessionTimeoutSeconds", strconv.FormatInt(plan.SessionTimeoutSeconds.ValueInt64(), 10))
	}
	if known(plan.Groups) {
		resp.Diagnostics.Append(setGroupsParam(ctx, params, plan.Groups)...)
	}
	user, err := r.client.SetUser(ctx, username, params)
	if err != nil {
		resp.Diagnostics.AddError("Error configuring user", err.Error())
		return
	}

	resp.Diagnostics.Append(applyUser(ctx, &plan, user)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, state.Username.ValueString())
	if technitium.IsUserNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading user", err.Error())
		return
	}

	resp.Diagnostics.Append(applyUser(ctx, &state, user)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	if plan.Username.ValueString() != state.Username.ValueString() {
		params.Set("newUser", plan.Username.ValueString())
	}
	if !plan.Password.Equal(state.Password) {
		params.Set("newPass", plan.Password.ValueString())
	}
	if known(plan.DisplayName) && !plan.DisplayName.Equal(state.DisplayName) {
		params.Set("displayName", plan.DisplayName.ValueString())
	}
	if !plan.Disabled.Equal(state.Disabled) {
		params.Set("disabled", strconv.FormatBool(plan.Disabled.ValueBool()))
	}
	if known(plan.SessionTimeoutSeconds) && !plan.SessionTimeoutSeconds.Equal(state.SessionTimeoutSeconds) {
		params.Set("sessionTimeoutSeconds", strconv.FormatInt(plan.SessionTimeoutSeconds.ValueInt64(), 10))
	}
	if known(plan.Groups) && !plan.Groups.Equal(state.Groups) {
		resp.Diagnostics.Append(setGroupsParam(ctx, params, plan.Groups)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.SetUser(ctx, state.Username.ValueString(), params)
	if err != nil {
		resp.Diagnostics.AddError("Error updating user", err.Error())
		return
	}

	resp.Diagnostics.Append(applyUser(ctx, &plan, user)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteUser(ctx, state.Username.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting user", err.Error())
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("username"), req.ID)...)
}

func setGroupsParam(ctx context.Context, params url.Values, groups types.Set) diag.Diagnostics {
	var names []string
	diags := groups.ElementsAs(ctx, &names, false)
	sort.Strings(names)
	// An empty value removes all memberships.
	params.Set("memberOfGroups", strings.Join(names, ","))
	return diags
}

func applyUser(ctx context.Context, m *userModel, u technitium.User) diag.Diagnostics {
	m.ID = types.StringValue(u.Username)
	m.Username = types.StringValue(u.Username)
	m.DisplayName = types.StringValue(u.DisplayName)
	m.Disabled = types.BoolValue(u.Disabled)
	m.SessionTimeoutSeconds = types.Int64Value(u.SessionTimeoutSeconds)
	m.TotpEnabled = types.BoolValue(u.TotpEnabled)
	groups, diags := types.SetValueFrom(ctx, types.StringType, nonNil(u.MemberOfGroups))
	m.Groups = groups
	return diags
}
