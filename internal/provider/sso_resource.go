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
	_ resource.ResourceWithConfigure   = &ssoResource{}
	_ resource.ResourceWithImportState = &ssoResource{}
)

func NewSsoResource() resource.Resource { return &ssoResource{} }

type ssoResource struct {
	client *technitium.Client
}

type ssoModel struct {
	ID                            types.String `tfsdk:"id"`
	Enabled                       types.Bool   `tfsdk:"enabled"`
	Authority                     types.String `tfsdk:"authority"`
	ClientID                      types.String `tfsdk:"client_id"`
	ClientSecret                  types.String `tfsdk:"client_secret"`
	MetadataAddress               types.String `tfsdk:"metadata_address"`
	AdditionalScopes              types.List   `tfsdk:"additional_scopes"`
	AllowSignup                   types.Bool   `tfsdk:"allow_signup"`
	AllowSignupOnlyForMappedUsers types.Bool   `tfsdk:"allow_signup_only_for_mapped_users"`
	GroupMap                      types.List   `tfsdk:"group_map"`
}

var ssoGroupMapType = types.ObjectType{AttrTypes: map[string]attr.Type{"remote_group": types.StringType, "local_group": types.StringType}}

// Scopes the server always requests.
var ssoBaseScopes = map[string]bool{"openid": true, "profile": true}

func (r *ssoResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sso"
}

func (r *ssoResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *ssoResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Description: desc,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
	}
	flag := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Optional: true, Computed: true, Description: desc,
			PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}
	}
	list := func(t attr.Type, desc string) schema.ListAttribute {
		return schema.ListAttribute{Optional: true, Computed: true, ElementType: t, Description: desc,
			PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}}
	}
	resp.Schema = schema.Schema{
		Description: "Manages OpenID Connect single sign-on for the web console (Technitium 15+). Singleton; unset attributes " +
			"follow the server. Changes may restart the web service; the provider waits for it to return.",
		Attributes: map[string]schema.Attribute{
			"id":                                 schema.StringAttribute{Computed: true, Description: "Always `sso`.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"enabled":                            flag("Enable SSO."),
			"authority":                          str("OIDC authority (issuer) URL, e.g. `https://auth.example.com/application/o/technitium/`."),
			"client_id":                          str("OIDC client ID."),
			"client_secret":                      schema.StringAttribute{Optional: true, Sensitive: true, Description: "OIDC client secret. Not readable from the server, so outside changes are not detected."},
			"metadata_address":                   str("Discovery URL, only if the provider doesn't use the standard one."),
			"additional_scopes":                  list(types.StringType, "Scopes requested besides `openid` and `profile` (always requested), e.g. `[\"email\", \"groups\"]`."),
			"allow_signup":                       flag("Create accounts for new SSO users on first sign-in."),
			"allow_signup_only_for_mapped_users": flag("Only sign up users in at least one mapped remote group."),
			"group_map":                          list(ssoGroupMapType, "Remote-to-local group mapping: objects with `remote_group` and `local_group`."),
		},
	}
}

func (r *ssoResource) params(ctx context.Context, plan, state *ssoModel) (url.Values, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := url.Values{}
	changed := func(a, b attr.Value) bool { return known(a) && (state == nil || !a.Equal(b)) }
	var prev ssoModel
	if state != nil {
		prev = *state
	}
	if changed(plan.Enabled, prev.Enabled) {
		p.Set("ssoEnabled", strconv.FormatBool(plan.Enabled.ValueBool()))
	}
	for _, f := range []struct {
		key        string
		plan, prev types.String
	}{
		{"ssoAuthority", plan.Authority, prev.Authority},
		{"ssoClientId", plan.ClientID, prev.ClientID},
		{"ssoMetadataAddress", plan.MetadataAddress, prev.MetadataAddress},
		{"ssoClientSecret", plan.ClientSecret, prev.ClientSecret},
	} {
		if changed(f.plan, f.prev) {
			p.Set(f.key, f.plan.ValueString())
		}
	}
	if changed(plan.AllowSignup, prev.AllowSignup) {
		p.Set("ssoAllowSignup", strconv.FormatBool(plan.AllowSignup.ValueBool()))
	}
	if changed(plan.AllowSignupOnlyForMappedUsers, prev.AllowSignupOnlyForMappedUsers) {
		p.Set("ssoAllowSignupOnlyForMappedUsers", strconv.FormatBool(plan.AllowSignupOnlyForMappedUsers.ValueBool()))
	}
	if changed(plan.AdditionalScopes, prev.AdditionalScopes) {
		var scopes []string
		diags.Append(plan.AdditionalScopes.ElementsAs(ctx, &scopes, false)...)
		p.Set("ssoScopes", strings.Join(scopes, "|"))
	}
	if changed(plan.GroupMap, prev.GroupMap) {
		var rows []struct {
			Remote string `tfsdk:"remote_group"`
			Local  string `tfsdk:"local_group"`
		}
		diags.Append(plan.GroupMap.ElementsAs(ctx, &rows, false)...)
		parts := make([]string, 0, len(rows)*2)
		for _, row := range rows {
			parts = append(parts, row.Remote, row.Local)
		}
		p.Set("ssoGroupMap", strings.Join(parts, "|"))
	}
	return p, diags
}

func applySso(m *ssoModel, c technitium.SsoConfig) diag.Diagnostics {
	var diags diag.Diagnostics
	deref := func(p *string) types.String {
		if p == nil {
			return types.StringValue("")
		}
		return types.StringValue(*p)
	}
	m.ID = types.StringValue("sso")
	m.Enabled = types.BoolValue(c.Enabled)
	m.Authority = deref(c.Authority)
	m.ClientID = deref(c.ClientID)
	m.MetadataAddress = deref(c.MetadataAddress)
	m.AllowSignup = types.BoolValue(c.AllowSignup)
	m.AllowSignupOnlyForMappedUsers = types.BoolValue(c.AllowSignupOnlyForMappedUsers)

	scopes := []attr.Value{}
	for _, s := range c.Scopes {
		if !ssoBaseScopes[s] {
			scopes = append(scopes, types.StringValue(s))
		}
	}
	m.AdditionalScopes = types.ListValueMust(types.StringType, scopes)
	m.GroupMap = objectList(&diags, ssoGroupMapType, len(c.GroupMap), func(i int) map[string]attr.Value {
		return map[string]attr.Value{"remote_group": types.StringValue(c.GroupMap[i].RemoteGroup), "local_group": types.StringValue(c.GroupMap[i].LocalGroup)}
	})
	return diags
}

func (r *ssoResource) write(ctx context.Context, plan, state *ssoModel) (technitium.SsoConfig, diag.Diagnostics) {
	params, diags := r.params(ctx, plan, state)
	if diags.HasError() {
		return technitium.SsoConfig{}, diags
	}
	if len(params) == 0 {
		cfg, err := r.client.GetSsoConfig(ctx)
		if err != nil {
			diags.AddError("Error reading SSO config", err.Error())
		}
		return cfg, diags
	}
	cfg, err := r.client.SetSsoConfig(ctx, params)
	if err != nil {
		diags.AddError("Error updating SSO config", err.Error())
	}
	return cfg, diags
}

func (r *ssoResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ssoModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, diags := r.write(ctx, &plan, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(applySso(&plan, cfg)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ssoResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ssoModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, err := r.client.GetSsoConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading SSO config", err.Error())
		return
	}
	resp.Diagnostics.Append(applySso(&state, cfg)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ssoResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ssoModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cfg, diags := r.write(ctx, &plan, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(applySso(&plan, cfg)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ssoResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("SSO config left unchanged", "technitium_sso was removed from state; the server keeps its SSO config.")
}

func (r *ssoResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "sso")...)
}
