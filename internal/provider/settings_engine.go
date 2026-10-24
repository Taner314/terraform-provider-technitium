package provider

import (
	"context"
	"fmt"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Server settings are one big /api/settings get/set pair. Each settingsSpec
// is a singleton resource owning a subset of fields; fields left unset in
// config follow the server, and destroying the resource leaves the server
// settings as they are.

type settingKind int

const (
	sStr settingKind = iota
	sInt
	sBool
	sStrList
	sIntList
	sQpmList
)

type settingField struct {
	attr      string
	key       string // API param and response key
	get       string // dotted response path when it differs from key
	kind      settingKind
	desc      string
	sensitive bool
	writeOnly bool // the server doesn't return it
}

type settingsSpec struct {
	name   string
	desc   string
	fields []settingField
}

var qpmType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"prefix":    types.Int64Type,
	"udp_limit": types.Int64Type,
	"tcp_limit": types.Int64Type,
}}

func (f settingField) elemType() attr.Type {
	switch f.kind {
	case sIntList:
		return types.Int64Type
	case sQpmList:
		return qpmType
	default:
		return types.StringType
	}
}

func (f settingField) schemaAttribute() schema.Attribute {
	switch f.kind {
	case sInt:
		return schema.Int64Attribute{Optional: true, Computed: true, Description: f.desc,
			PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}
	case sBool:
		return schema.BoolAttribute{Optional: true, Computed: true, Description: f.desc,
			PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}
	case sStrList, sIntList:
		elem := attr.Type(types.StringType)
		if f.kind == sIntList {
			elem = types.Int64Type
		}
		return schema.ListAttribute{Optional: true, Computed: true, ElementType: elem, Description: f.desc,
			PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}}
	case sQpmList:
		return schema.ListAttribute{Optional: true, Computed: true, ElementType: qpmType, Description: f.desc,
			PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}}
	default:
		return schema.StringAttribute{Optional: true, Computed: true, Description: f.desc, Sensitive: f.sensitive,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
	}
}

// lookup follows a dotted path in the settings response.
func lookup(s technitium.Settings, p string) any {
	var cur any = map[string]any(s)
	for _, part := range strings.Split(p, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

// fromSettings converts a response value; null becomes the empty value so
// "unset" and "empty" compare equal.
func (f settingField) fromSettings(s technitium.Settings) (attr.Value, diag.Diagnostics) {
	p := f.key
	if f.get != "" {
		p = f.get
	}
	raw := lookup(s, p)

	switch f.kind {
	case sInt:
		n, _ := raw.(float64)
		return types.Int64Value(int64(n)), nil
	case sBool:
		b, _ := raw.(bool)
		return types.BoolValue(b), nil
	case sStrList, sIntList:
		items, _ := raw.([]any)
		elems := make([]attr.Value, 0, len(items))
		for _, it := range items {
			if f.kind == sIntList {
				n, _ := it.(float64)
				elems = append(elems, types.Int64Value(int64(n)))
			} else {
				elems = append(elems, types.StringValue(fmt.Sprint(it)))
			}
		}
		return types.ListValue(f.elemType(), elems)
	case sQpmList:
		items, _ := raw.([]any)
		elems := make([]attr.Value, 0, len(items))
		var diags diag.Diagnostics
		for _, it := range items {
			m, _ := it.(map[string]any)
			num := func(k string) attr.Value { n, _ := m[k].(float64); return types.Int64Value(int64(n)) }
			obj, d := types.ObjectValue(qpmType.AttrTypes, map[string]attr.Value{
				"prefix": num("prefix"), "udp_limit": num("udpLimit"), "tcp_limit": num("tcpLimit"),
			})
			diags.Append(d...)
			elems = append(elems, obj)
		}
		v, d := types.ListValue(qpmType, elems)
		diags.Append(d...)
		return v, diags
	default:
		if raw == nil {
			if f.key == "proxyType" {
				return types.StringValue("None"), nil
			}
			return types.StringValue(""), nil
		}
		return types.StringValue(fmt.Sprint(raw)), nil
	}
}

// encode renders a planned value as a settings/set param. Empty lists are
// "false", which the API documents as "clear".
func (f settingField) encode(ctx context.Context, v attr.Value) (string, diag.Diagnostics) {
	switch x := v.(type) {
	case types.String:
		return x.ValueString(), nil
	case types.Int64:
		return strconv.FormatInt(x.ValueInt64(), 10), nil
	case types.Bool:
		return strconv.FormatBool(x.ValueBool()), nil
	case types.List:
		if len(x.Elements()) == 0 {
			return "false", nil
		}
		switch f.kind {
		case sIntList:
			var items []int64
			diags := x.ElementsAs(ctx, &items, false)
			parts := make([]string, len(items))
			for i, n := range items {
				parts[i] = strconv.FormatInt(n, 10)
			}
			return strings.Join(parts, ","), diags
		case sQpmList:
			var items []struct {
				Prefix   int64 `tfsdk:"prefix"`
				UdpLimit int64 `tfsdk:"udp_limit"`
				TcpLimit int64 `tfsdk:"tcp_limit"`
			}
			diags := x.ElementsAs(ctx, &items, false)
			parts := make([]string, 0, len(items)*3)
			for _, it := range items {
				parts = append(parts, strconv.FormatInt(it.Prefix, 10), strconv.FormatInt(it.UdpLimit, 10), strconv.FormatInt(it.TcpLimit, 10))
			}
			return strings.Join(parts, "|"), diags
		default:
			var items []string
			diags := x.ElementsAs(ctx, &items, false)
			return strings.Join(items, ","), diags
		}
	}
	return "", nil
}

func (f settingField) target() any {
	switch f.kind {
	case sInt:
		return &types.Int64{}
	case sBool:
		return &types.Bool{}
	case sStrList, sIntList, sQpmList:
		return &types.List{}
	default:
		return &types.String{}
	}
}

func deref(p any) attr.Value {
	switch x := p.(type) {
	case *types.Int64:
		return *x
	case *types.Bool:
		return *x
	case *types.List:
		return *x
	case *types.String:
		return *x
	}
	return nil
}

func (s *settingsSpec) values(ctx context.Context, g attrGetter) (map[string]attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	vals := map[string]attr.Value{}
	for _, f := range s.fields {
		t := f.target()
		diags.Append(g.GetAttribute(ctx, path.Root(f.attr), t)...)
		vals[f.attr] = deref(t)
	}
	return vals, diags
}

type settingsResource struct {
	client *technitium.Client
	spec   *settingsSpec
}

var (
	_ resource.ResourceWithConfigure   = &settingsResource{}
	_ resource.ResourceWithImportState = &settingsResource{}
	_ resource.ResourceWithModifyPlan  = &settingsResource{}
)

func newSettingsResource(spec *settingsSpec) func() resource.Resource {
	return func() resource.Resource { return &settingsResource{spec: spec} }
}

func (r *settingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_settings_" + r.spec.name
}

func (r *settingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *settingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, Description: "Always `settings`.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
	}
	for _, f := range r.spec.fields {
		attrs[f.attr] = f.schemaAttribute()
	}
	resp.Schema = schema.Schema{
		Description: r.spec.desc + "\n\nThis is a singleton: declare it once. Attributes left unset follow the server's " +
			"current value. Destroying the resource only removes it from state; server settings are not reset.",
		Attributes: attrs,
	}
}

// apply sends changed fields (all known fields when state is nil) and
// writes the resulting server values to st.
func (r *settingsResource) apply(ctx context.Context, plan, state map[string]attr.Value, st attrSetter) diag.Diagnostics {
	var diags diag.Diagnostics
	current, err := r.client.GetSettings(ctx)
	if err != nil {
		diags.AddError("Error reading settings", err.Error())
		return diags
	}

	params := url.Values{}
	for _, f := range r.spec.fields {
		p := plan[f.attr]
		if !known(p) {
			continue
		}
		var prev attr.Value
		if state != nil {
			prev = state[f.attr]
		} else if !f.writeOnly {
			prev, _ = f.fromSettings(current)
		}
		if prev != nil && known(prev) && p.Equal(prev) {
			continue
		}
		v, d := f.encode(ctx, p)
		diags.Append(d...)
		params.Set(f.key, v)
	}
	if diags.HasError() {
		return diags
	}

	if len(params) > 0 {
		current, err = r.client.SetSettings(ctx, params)
		if err != nil {
			diags.AddError("Error updating settings", err.Error())
			return diags
		}
	}
	diags.Append(r.write(ctx, st, current, plan)...)
	return diags
}

// write stores server values; write-only fields keep the planned value.
func (r *settingsResource) write(ctx context.Context, st attrSetter, s technitium.Settings, keep map[string]attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(st.SetAttribute(ctx, path.Root("id"), "settings")...)
	for _, f := range r.spec.fields {
		var v attr.Value
		if f.writeOnly {
			v = keep[f.attr]
			if !known(v) {
				v = types.StringValue("")
			}
		} else {
			var d diag.Diagnostics
			v, d = f.fromSettings(s)
			diags.Append(d...)
		}
		diags.Append(st.SetAttribute(ctx, path.Root(f.attr), v)...)
	}
	return diags
}

// ModifyPlan: when an area changes, the server may adjust related fields
// (e.g. proxy type None clears the address), so unconfigured fields become
// unknown instead of carrying their old state value.
func (r *settingsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	plan, diags := r.spec.values(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	state, diags := r.spec.values(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	config, diags := r.spec.values(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	changed := false
	for _, f := range r.spec.fields {
		if known(plan[f.attr]) && !plan[f.attr].Equal(state[f.attr]) {
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	for _, f := range r.spec.fields {
		if f.writeOnly || !config[f.attr].IsNull() {
			continue
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(f.attr), unknownOf(f))...)
	}
}

func unknownOf(f settingField) attr.Value {
	switch f.kind {
	case sInt:
		return types.Int64Unknown()
	case sBool:
		return types.BoolUnknown()
	case sStrList, sIntList, sQpmList:
		return types.ListUnknown(f.elemType())
	default:
		return types.StringUnknown()
	}
}

func (r *settingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	plan, diags := r.spec.values(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, plan, nil, &resp.State)...)
}

func (r *settingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state, diags := r.spec.values(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetSettings(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading settings", err.Error())
		return
	}
	resp.Diagnostics.Append(r.write(ctx, &resp.State, s, state)...)
}

func (r *settingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan, diags := r.spec.values(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	state, diags := r.spec.values(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, plan, state, &resp.State)...)
}

func (r *settingsResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning("Settings left unchanged",
		"technitium_settings_"+r.spec.name+" was removed from state; the server keeps its current settings.")
}

func (r *settingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "settings" {
		resp.Diagnostics.AddError("Invalid import ID", `Settings resources are imported with the ID "settings".`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "settings")...)
}
