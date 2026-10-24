package provider

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var permissionSections = []string{
	"Dashboard", "Zones", "Cache", "Allowed", "Blocked", "Apps", "DnsClient",
	"Settings", "DhcpServer", "Administration", "Logs",
}

// permissionResource manages one user or group entry in a permission table:
// a server section (technitium_permission) or a zone
// (technitium_zone_permission). Other entries are left untouched.
type permissionResource struct {
	client *technitium.Client
	zone   bool
}

var (
	_ resource.ResourceWithConfigure   = &permissionResource{}
	_ resource.ResourceWithImportState = &permissionResource{}
)

func NewPermissionResource() resource.Resource     { return &permissionResource{} }
func NewZonePermissionResource() resource.Resource { return &permissionResource{zone: true} }

type permissionModel struct {
	ID        types.String `tfsdk:"id"`
	Scope     types.String `tfsdk:"-"`
	User      types.String `tfsdk:"user"`
	Group     types.String `tfsdk:"group"`
	CanView   types.Bool   `tfsdk:"can_view"`
	CanModify types.Bool   `tfsdk:"can_modify"`
	CanDelete types.Bool   `tfsdk:"can_delete"`
}

func (r *permissionResource) scopeAttr() string {
	if r.zone {
		return "zone"
	}
	return "section"
}

func (r *permissionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	if r.zone {
		resp.TypeName = req.ProviderTypeName + "_zone_permission"
	} else {
		resp.TypeName = req.ProviderTypeName + "_permission"
	}
}

func (r *permissionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *permissionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	principal := []validator.String{stringvalidator.ExactlyOneOf(path.MatchRoot("user"), path.MatchRoot("group"))}

	scope := schema.StringAttribute{
		Required: true, PlanModifiers: replace,
		Description: "Section: `" + strings.Join(permissionSections, "`, `") + "`.",
		Validators:  []validator.String{stringvalidator.OneOf(permissionSections...)},
	}
	desc := "Grants a user or group access to a server section. Only this entry is managed; others in the section are left alone."
	if r.zone {
		scope = schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Zone name."}
		desc = "Grants a user or group access to one zone. Only this entry is managed; others on the zone are left alone."
	}

	resp.Schema = schema.Schema{
		Description: desc,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "`<" + r.scopeAttr() + ">/user/<name>` or `<" + r.scopeAttr() + ">/group/<name>`; also the import ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			r.scopeAttr(): scope,
			"user":        schema.StringAttribute{Optional: true, PlanModifiers: replace, Validators: principal, Description: "Username to grant to."},
			"group":       schema.StringAttribute{Optional: true, PlanModifiers: replace, Validators: principal, Description: "Group to grant to (`Everyone` included)."},
			"can_view":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "View permission."},
			"can_modify":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Modify permission."},
			"can_delete":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Delete permission."},
		},
	}
}

func (r *permissionResource) get(ctx context.Context, g attrGetter) (permissionModel, bool) {
	var m permissionModel
	diags := g.GetAttribute(ctx, path.Root("id"), &m.ID)
	diags.Append(g.GetAttribute(ctx, path.Root(r.scopeAttr()), &m.Scope)...)
	diags.Append(g.GetAttribute(ctx, path.Root("user"), &m.User)...)
	diags.Append(g.GetAttribute(ctx, path.Root("group"), &m.Group)...)
	diags.Append(g.GetAttribute(ctx, path.Root("can_view"), &m.CanView)...)
	diags.Append(g.GetAttribute(ctx, path.Root("can_modify"), &m.CanModify)...)
	diags.Append(g.GetAttribute(ctx, path.Root("can_delete"), &m.CanDelete)...)
	return m, !diags.HasError()
}

func (r *permissionResource) set(ctx context.Context, st attrSetter, m permissionModel) {
	kind, name := "group", m.Group.ValueString()
	if known(m.User) && m.User.ValueString() != "" {
		kind, name = "user", m.User.ValueString()
	}
	st.SetAttribute(ctx, path.Root("id"), m.Scope.ValueString()+"/"+kind+"/"+name)
	st.SetAttribute(ctx, path.Root(r.scopeAttr()), m.Scope)
	st.SetAttribute(ctx, path.Root("user"), m.User)
	st.SetAttribute(ctx, path.Root("group"), m.Group)
	st.SetAttribute(ctx, path.Root("can_view"), m.CanView)
	st.SetAttribute(ctx, path.Root("can_modify"), m.CanModify)
	st.SetAttribute(ctx, path.Root("can_delete"), m.CanDelete)
}

func (r *permissionResource) section(m permissionModel) (section, zone string) {
	if r.zone {
		return "Zones", m.Scope.ValueString()
	}
	return m.Scope.ValueString(), ""
}

// entry returns the slice holding the principal and its matcher.
func entryMatch(m permissionModel) (users bool, name string) {
	if known(m.User) && m.User.ValueString() != "" {
		return true, m.User.ValueString()
	}
	return false, m.Group.ValueString()
}

func findEntry(p technitium.Permissions, m permissionModel) (technitium.Permission, bool) {
	users, name := entryMatch(m)
	list := p.GroupPermissions
	if users {
		list = p.UserPermissions
	}
	for _, e := range list {
		n := e.Name
		if users {
			n = e.Username
		}
		if strings.EqualFold(n, name) {
			return e, true
		}
	}
	return technitium.Permission{}, false
}

// write upserts (or with remove, deletes) the entry for m.
func (r *permissionResource) write(ctx context.Context, m permissionModel, remove bool) error {
	unlock := technitium.LockSettings()
	defer unlock()

	section, zone := r.section(m)
	p, err := r.client.GetPermissions(ctx, section, zone)
	if err != nil {
		return err
	}
	users, name := entryMatch(m)
	entry := technitium.Permission{CanView: m.CanView.ValueBool(), CanModify: m.CanModify.ValueBool(), CanDelete: m.CanDelete.ValueBool()}
	if users {
		entry.Username = name
	} else {
		entry.Name = name
	}

	upsert := func(list []technitium.Permission) []technitium.Permission {
		out := make([]technitium.Permission, 0, len(list)+1)
		for _, e := range list {
			n := e.Name
			if users {
				n = e.Username
			}
			if !strings.EqualFold(n, name) {
				out = append(out, e)
			}
		}
		if !remove {
			out = append(out, entry)
		}
		return out
	}
	if users {
		p.UserPermissions = upsert(p.UserPermissions)
	} else {
		p.GroupPermissions = upsert(p.GroupPermissions)
	}
	p.Section = section
	return r.client.SetPermissions(ctx, p, zone)
}

func (r *permissionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	m, ok := r.get(ctx, req.Plan)
	if !ok {
		resp.Diagnostics.AddError("Invalid plan", "could not read planned permission")
		return
	}
	if err := r.write(ctx, m, false); err != nil {
		resp.Diagnostics.AddError("Error setting permission", err.Error())
		return
	}
	r.set(ctx, &resp.State, m)
}

func (r *permissionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	m, ok := r.get(ctx, req.State)
	if !ok {
		resp.Diagnostics.AddError("Invalid state", "could not read permission state")
		return
	}
	section, zone := r.section(m)
	p, err := r.client.GetPermissions(ctx, section, zone)
	if technitium.IsZoneNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading permission", err.Error())
		return
	}
	e, found := findEntry(p, m)
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	m.CanView, m.CanModify, m.CanDelete = types.BoolValue(e.CanView), types.BoolValue(e.CanModify), types.BoolValue(e.CanDelete)
	r.set(ctx, &resp.State, m)
}

func (r *permissionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	m, ok := r.get(ctx, req.Plan)
	if !ok {
		resp.Diagnostics.AddError("Invalid plan", "could not read planned permission")
		return
	}
	if err := r.write(ctx, m, false); err != nil {
		resp.Diagnostics.AddError("Error updating permission", err.Error())
		return
	}
	r.set(ctx, &resp.State, m)
}

func (r *permissionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	m, ok := r.get(ctx, req.State)
	if !ok {
		resp.Diagnostics.AddError("Invalid state", "could not read permission state")
		return
	}
	err := r.write(ctx, m, true)
	if err != nil && !technitium.IsZoneNotFound(err) {
		resp.Diagnostics.AddError("Error removing permission", err.Error())
	}
}

func (r *permissionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 || (parts[1] != "user" && parts[1] != "group") || parts[0] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <%s>/user/<name> or <%s>/group/<name>, got %q", r.scopeAttr(), r.scopeAttr(), req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(r.scopeAttr()), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(parts[1]), parts[2])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
