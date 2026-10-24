package provider

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The record engine implements one Terraform resource per DNS record type
// from a declarative recordSpec. Technitium records have no IDs, so a record
// is identified by zone + name + type + the spec's key fields, compared after
// normalisation (the server rewrites case, trailing dots, IPv6 and hex).

type fieldKind int

const (
	kString fieldKind = iota
	kInt
	kBool
	kStringMap
)

type recordField struct {
	attr     string // Terraform attribute name
	param    string // API param holding the current/added value
	newParam string // API param for the new value on update; "" = param is sent with the planned value
	rdata    string // key in rData; "" = not returned by the server (state keeps the planned value)
	kind     fieldKind
	key      bool // part of the record identity
	del      bool // sent on delete to select the record
	optional bool // Optional+Computed with a default instead of Required
	defStr   string
	defInt   int64
	defBool  bool
	norm     func(string) string
	desc     string
	sens     bool
}

type recordSpec struct {
	typ       string
	desc      string
	singleton bool // at most one record of this type per name (CNAME, DNAME, APP)
	noTTL     bool // the server stores TTL 0 (FWD); keep the configured value
	fields    []recordField
	// extraParams adds fixed params on add/update/delete.
	extraParams func(params url.Values, op string)
	// fixRData adjusts server rData before comparison, given planned/state values.
	fixRData func(rdata map[string]any, vals map[string]attr.Value)
}

// replaces reports whether changing f forces a new record.
func (f recordField) replaces() bool { return f.key && f.newParam == "" }

func (f recordField) normalize(s string) string {
	if f.norm == nil {
		return s
	}
	return f.norm(s)
}

// recordVals is the record's Terraform-side state.
type recordVals struct {
	zone, name string
	ttl        types.Int64
	comments   string
	disabled   bool
	expiryTTL  int64
	fields     map[string]attr.Value
}

type attrGetter interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

type attrSetter interface {
	SetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

func (s *recordSpec) get(ctx context.Context, g attrGetter) (recordVals, diag.Diagnostics) {
	var diags diag.Diagnostics
	var zone, name, comments types.String
	var disabled types.Bool
	var expiry types.Int64
	v := recordVals{fields: map[string]attr.Value{}}

	diags.Append(g.GetAttribute(ctx, path.Root("zone"), &zone)...)
	diags.Append(g.GetAttribute(ctx, path.Root("name"), &name)...)
	diags.Append(g.GetAttribute(ctx, path.Root("ttl"), &v.ttl)...)
	diags.Append(g.GetAttribute(ctx, path.Root("comments"), &comments)...)
	diags.Append(g.GetAttribute(ctx, path.Root("disabled"), &disabled)...)
	diags.Append(g.GetAttribute(ctx, path.Root("expiry_ttl"), &expiry)...)
	v.zone, v.name, v.comments = zone.ValueString(), name.ValueString(), comments.ValueString()
	v.disabled, v.expiryTTL = disabled.ValueBool(), expiry.ValueInt64()

	for _, f := range s.fields {
		var val attr.Value
		switch f.kind {
		case kString:
			var x types.String
			diags.Append(g.GetAttribute(ctx, path.Root(f.attr), &x)...)
			val = x
		case kInt:
			var x types.Int64
			diags.Append(g.GetAttribute(ctx, path.Root(f.attr), &x)...)
			val = x
		case kBool:
			var x types.Bool
			diags.Append(g.GetAttribute(ctx, path.Root(f.attr), &x)...)
			val = x
		case kStringMap:
			var x types.Map
			diags.Append(g.GetAttribute(ctx, path.Root(f.attr), &x)...)
			val = x
		}
		v.fields[f.attr] = val
	}
	return v, diags
}

func (s *recordSpec) set(ctx context.Context, st attrSetter, v recordVals) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(st.SetAttribute(ctx, path.Root("id"), s.id(v))...)
	diags.Append(st.SetAttribute(ctx, path.Root("fqdn"), fqdn(v.name, v.zone))...)
	diags.Append(st.SetAttribute(ctx, path.Root("zone"), v.zone)...)
	diags.Append(st.SetAttribute(ctx, path.Root("name"), v.name)...)
	diags.Append(st.SetAttribute(ctx, path.Root("ttl"), v.ttl)...)
	diags.Append(st.SetAttribute(ctx, path.Root("comments"), v.comments)...)
	diags.Append(st.SetAttribute(ctx, path.Root("disabled"), v.disabled)...)
	diags.Append(st.SetAttribute(ctx, path.Root("expiry_ttl"), v.expiryTTL)...)
	for _, f := range s.fields {
		diags.Append(st.SetAttribute(ctx, path.Root(f.attr), v.fields[f.attr])...)
	}
	return diags
}

// fqdn turns a zone-relative name ("@" for the apex) into a domain name.
func fqdn(name, zone string) string {
	zone = strings.TrimSuffix(zone, ".")
	if name == "@" || name == "" {
		return zone
	}
	return name + "." + zone
}

// relativeName is the inverse of fqdn.
func relativeName(domain, zone string) string {
	d, z := strings.ToLower(strings.TrimSuffix(domain, ".")), strings.ToLower(strings.TrimSuffix(zone, "."))
	if d == z {
		return "@"
	}
	if strings.HasSuffix(d, "."+z) {
		return domain[:len(d)-len(z)-1]
	}
	return domain
}

// keyFields lists the identity fields in id order.
func (s *recordSpec) keyFields() []recordField {
	var keys []recordField
	if s.singleton {
		return keys
	}
	for _, f := range s.fields {
		if f.key {
			keys = append(keys, f)
		}
	}
	return keys
}

// id is "<zone>/<name>[/<key values separated by spaces>]", which is also the
// import ID.
func (s *recordSpec) id(v recordVals) string {
	id := strings.TrimSuffix(v.zone, ".") + "/" + v.name
	keys := s.keyFields()
	if len(keys) == 0 {
		return id
	}
	parts := make([]string, 0, len(keys))
	for _, f := range keys {
		parts = append(parts, valueString(v.fields[f.attr]))
	}
	return id + "/" + strings.Join(parts, " ")
}

// parseID fills zone, name and key fields from an import ID.
func (s *recordSpec) parseID(id string) (recordVals, error) {
	keys := s.keyFields()
	n := 2
	if len(keys) > 0 {
		n = 3
	}
	parts := strings.SplitN(id, "/", n)
	if len(parts) != n || parts[0] == "" || parts[1] == "" {
		return recordVals{}, fmt.Errorf("expected %s, got %q", s.idFormat(), id)
	}
	v := recordVals{zone: parts[0], name: parts[1], fields: map[string]attr.Value{}}
	if len(keys) == 0 {
		return v, nil
	}
	// The last key may contain spaces (TXT text, CAA value, URIs).
	values := strings.SplitN(parts[2], " ", len(keys))
	if len(values) != len(keys) {
		return recordVals{}, fmt.Errorf("expected %s, got %q", s.idFormat(), id)
	}
	for i, f := range keys {
		val, err := parseValue(f, values[i])
		if err != nil {
			return recordVals{}, fmt.Errorf("%s: %w", f.attr, err)
		}
		v.fields[f.attr] = val
	}
	return v, nil
}

func (s *recordSpec) idFormat() string {
	keys := s.keyFields()
	if len(keys) == 0 {
		return "<zone>/<name>"
	}
	names := make([]string, 0, len(keys))
	for _, f := range keys {
		names = append(names, "<"+f.attr+">")
	}
	return "<zone>/<name>/" + strings.Join(names, " ")
}

func parseValue(f recordField, s string) (attr.Value, error) {
	switch f.kind {
	case kInt:
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		return types.Int64Value(i), nil
	case kBool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, err
		}
		return types.BoolValue(b), nil
	default:
		return types.StringValue(s), nil
	}
}

func valueString(v attr.Value) string {
	switch x := v.(type) {
	case types.String:
		return x.ValueString()
	case types.Int64:
		return strconv.FormatInt(x.ValueInt64(), 10)
	case types.Bool:
		return strconv.FormatBool(x.ValueBool())
	}
	return ""
}

func stringOf(v attr.Value) string {
	if s, ok := v.(types.String); ok {
		return s.ValueString()
	}
	return ""
}

func known(v attr.Value) bool { return v != nil && !v.IsNull() && !v.IsUnknown() }

// addParam encodes v as API param name. Empty strings are skipped (v15
// treats them as explicit values); empty maps are sent as "false" (clear).
func addParam(params url.Values, name string, v attr.Value) {
	if name == "" || !known(v) {
		return
	}
	switch x := v.(type) {
	case types.String:
		addIfSet(params, name, x.ValueString())
	case types.Int64:
		params.Set(name, strconv.FormatInt(x.ValueInt64(), 10))
	case types.Bool:
		params.Set(name, strconv.FormatBool(x.ValueBool()))
	case types.Map:
		params.Set(name, encodeSvcParams(x))
	}
}

func addIfSet(params url.Values, key, value string) {
	if value != "" {
		params.Set(key, value)
	}
}

// encodeSvcParams renders a map as Technitium's "key|value|key|value" form.
func encodeSvcParams(m types.Map) string {
	elems := m.Elements()
	if len(elems) == 0 {
		return "false"
	}
	keys := make([]string, 0, len(elems))
	for k := range elems {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		parts = append(parts, k, stringOf(elems[k]))
	}
	return strings.Join(parts, "|")
}

func (s *recordSpec) baseParams(domain, zone string) url.Values {
	params := url.Values{}
	params.Set("domain", domain)
	params.Set("zone", strings.TrimSuffix(zone, "."))
	params.Set("type", s.typ)
	return params
}

func (s *recordSpec) addParams(v recordVals) url.Values {
	params := s.baseParams(fqdn(v.name, v.zone), v.zone)
	if known(v.ttl) {
		params.Set("ttl", strconv.FormatInt(v.ttl.ValueInt64(), 10))
	}
	addIfSet(params, "comments", v.comments)
	if v.expiryTTL > 0 {
		params.Set("expiryTtl", strconv.FormatInt(v.expiryTTL, 10))
	}
	for _, f := range s.fields {
		addParam(params, f.param, v.fields[f.attr])
	}
	if s.extraParams != nil {
		s.extraParams(params, "add")
	}
	return params
}

func (s *recordSpec) updateParams(state, plan recordVals) url.Values {
	params := s.baseParams(fqdn(state.name, state.zone), state.zone)
	if state.name != plan.name {
		params.Set("newDomain", fqdn(plan.name, plan.zone))
	}
	ttl := plan.ttl
	if !known(ttl) {
		ttl = state.ttl
	}
	if known(ttl) {
		params.Set("ttl", strconv.FormatInt(ttl.ValueInt64(), 10))
	}
	// Always sent so that removing comments/expiry clears them.
	params.Set("comments", plan.comments)
	params.Set("disable", strconv.FormatBool(plan.disabled))
	params.Set("expiryTtl", strconv.FormatInt(plan.expiryTTL, 10))

	for _, f := range s.fields {
		if f.newParam != "" {
			addParam(params, f.param, state.fields[f.attr])
			addParam(params, f.newParam, plan.fields[f.attr])
		} else {
			addParam(params, f.param, plan.fields[f.attr])
		}
	}
	if s.extraParams != nil {
		s.extraParams(params, "update")
	}
	return params
}

func (s *recordSpec) deleteParams(v recordVals) url.Values {
	params := s.baseParams(fqdn(v.name, v.zone), v.zone)
	for _, f := range s.fields {
		if f.del {
			addParam(params, f.param, v.fields[f.attr])
		}
	}
	if s.extraParams != nil {
		s.extraParams(params, "delete")
	}
	return params
}

// rdataValue converts a JSON rData value into the field's attr type.
func rdataValue(f recordField, raw any) (attr.Value, bool) {
	switch f.kind {
	case kString:
		switch x := raw.(type) {
		case string:
			return types.StringValue(x), true
		case nil:
			return types.StringValue(""), true
		case float64:
			return types.StringValue(strconv.FormatFloat(x, 'f', -1, 64)), true
		}
	case kInt:
		if x, ok := raw.(float64); ok {
			return types.Int64Value(int64(x)), true
		}
	case kBool:
		if x, ok := raw.(bool); ok {
			return types.BoolValue(x), true
		}
	case kStringMap:
		m, ok := raw.(map[string]any)
		if !ok && raw != nil {
			return nil, false
		}
		elems := map[string]attr.Value{}
		for k, val := range m {
			elems[k] = types.StringValue(fmt.Sprint(val))
		}
		return types.MapValueMust(types.StringType, elems), true
	}
	return nil, false
}

// equalField compares a Terraform value with a server value, normalised.
func equalField(f recordField, a, b attr.Value) bool {
	if !known(a) || !known(b) {
		return false
	}
	switch f.kind {
	case kString:
		return f.normalize(stringOf(a)) == f.normalize(stringOf(b))
	case kStringMap:
		am, aok := a.(types.Map)
		bm, bok := b.(types.Map)
		if !aok || !bok || len(am.Elements()) != len(bm.Elements()) {
			return false
		}
		belems := bm.Elements()
		for k, av := range am.Elements() {
			bv, ok := belems[strings.ToLower(k)]
			if !ok {
				bv, ok = belems[k]
			}
			if !ok || stringOf(av) != stringOf(bv) {
				return false
			}
		}
		return true
	default:
		return a.Equal(b)
	}
}

// matches reports whether server record r is the one described by v.
func (s *recordSpec) matches(r technitium.Record, v recordVals) bool {
	if !strings.EqualFold(r.Type, s.typ) {
		return false
	}
	if s.singleton {
		return true
	}
	rdata := s.normRData(r.RData, v.fields)
	for _, f := range s.fields {
		if !f.key {
			continue
		}
		got, ok := rdataValue(f, rdata[f.rdata])
		if !ok || !equalField(f, v.fields[f.attr], got) {
			return false
		}
	}
	return true
}

func (s *recordSpec) normRData(rdata map[string]any, vals map[string]attr.Value) map[string]any {
	if s.fixRData == nil {
		return rdata
	}
	cp := make(map[string]any, len(rdata))
	for k, v := range rdata {
		cp[k] = v
	}
	s.fixRData(cp, vals)
	return cp
}

// merge folds server record r into v. Values equal after normalisation keep
// their configured spelling so plans stay empty.
func (s *recordSpec) merge(v recordVals, r technitium.Record) recordVals {
	out := recordVals{
		zone:      v.zone,
		name:      v.name,
		ttl:       types.Int64Value(r.TTL),
		comments:  r.Comments,
		disabled:  r.Disabled,
		expiryTTL: r.ExpiryTTL,
		fields:    map[string]attr.Value{},
	}
	if s.noTTL {
		out.ttl = v.ttl
		if !known(out.ttl) {
			out.ttl = types.Int64Value(0)
		}
	}
	if !strings.EqualFold(fqdn(v.name, v.zone), r.Name) {
		out.name = relativeName(r.Name, v.zone)
	}
	rdata := s.normRData(r.RData, v.fields)
	for _, f := range s.fields {
		cur := v.fields[f.attr]
		raw, present := rdata[f.rdata]
		if f.rdata == "" || !present {
			out.fields[f.attr] = s.defaultIfNull(f, cur)
			continue
		}
		got, ok := rdataValue(f, raw)
		if !ok || equalField(f, cur, got) {
			out.fields[f.attr] = s.defaultIfNull(f, cur)
			continue
		}
		out.fields[f.attr] = got
	}
	return out
}

// defaultIfNull fills fields that were never set (e.g. after import).
func (s *recordSpec) defaultIfNull(f recordField, v attr.Value) attr.Value {
	if known(v) {
		return v
	}
	switch f.kind {
	case kString:
		return types.StringValue(f.defStr)
	case kInt:
		return types.Int64Value(f.defInt)
	case kBool:
		return types.BoolValue(f.defBool)
	case kStringMap:
		return types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	return v
}

func (s *recordSpec) schema() schema.Schema {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Record identifier and import ID: `" + s.idFormat() + "`.",
		},
		"zone": schema.StringAttribute{
			Required:      true,
			Description:   "Authoritative zone the record belongs to.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "Record name relative to the zone; `@` for the zone apex.",
		},
		"fqdn": schema.StringAttribute{
			Computed:    true,
			Description: "Fully qualified domain name of the record.",
		},
		"ttl": schema.Int64Attribute{
			Optional:      true,
			Computed:      true,
			Description:   "TTL in seconds. Defaults to the server's default record TTL. All records of one type at the same name form an RRset and share a single TTL, so give them the same value.",
			PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
		},
		"comments": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(""),
			Description: "Free-form comments stored with the record.",
		},
		"disabled": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(false),
			Description: "Whether the record is disabled (kept but not served).",
		},
		"expiry_ttl": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(0),
			Description: "Seconds after the last modification at which the server deletes the record; 0 disables expiry.",
		},
	}

	for _, f := range s.fields {
		attrs[f.attr] = f.schemaAttribute()
	}

	return schema.Schema{
		Description: s.desc + "\n\nRecords are matched by zone, name and " + s.identityDescription() + ".",
		Attributes:  attrs,
	}
}

func (s *recordSpec) identityDescription() string {
	keys := s.keyFields()
	if len(keys) == 0 {
		return "type (one " + s.typ + " record per name)"
	}
	names := make([]string, 0, len(keys))
	for _, f := range keys {
		names = append(names, "`"+f.attr+"`")
	}
	return strings.Join(names, ", ")
}

func (f recordField) schemaAttribute() schema.Attribute {
	required, optional, computed := !f.optional, f.optional, f.optional
	switch f.kind {
	case kString:
		a := schema.StringAttribute{Required: required, Optional: optional, Computed: computed, Description: f.desc, Sensitive: f.sens}
		if f.optional {
			a.Default = stringdefault.StaticString(f.defStr)
		}
		if f.replaces() {
			a.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
		}
		return a
	case kInt:
		a := schema.Int64Attribute{Required: required, Optional: optional, Computed: computed, Description: f.desc}
		if f.optional {
			a.Default = int64default.StaticInt64(f.defInt)
		}
		if f.replaces() {
			a.PlanModifiers = []planmodifier.Int64{int64planmodifier.RequiresReplace()}
		}
		return a
	case kBool:
		a := schema.BoolAttribute{Required: required, Optional: optional, Computed: computed, Description: f.desc}
		if f.optional {
			a.Default = booldefault.StaticBool(f.defBool)
		}
		return a
	default:
		a := schema.MapAttribute{ElementType: types.StringType, Required: required, Optional: optional, Computed: computed, Description: f.desc}
		if f.optional {
			a.Default = mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{}))
		}
		return a
	}
}

// recordResource is the Terraform resource for one recordSpec.
type recordResource struct {
	client *technitium.Client
	spec   *recordSpec
}

var (
	_ resource.ResourceWithConfigure   = &recordResource{}
	_ resource.ResourceWithImportState = &recordResource{}
	_ resource.ResourceWithModifyPlan  = &recordResource{}
)

func newRecordResource(spec *recordSpec) func() resource.Resource {
	return func() resource.Resource { return &recordResource{spec: spec} }
}

func (r *recordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record_" + strings.ToLower(r.spec.typ)
}

func (r *recordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.spec.schema()
}

func (r *recordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

// ModifyPlan precomputes id and fqdn so plans don't show them as unknown.
func (r *recordResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	plan, diags := r.spec.get(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var zone, name types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("zone"), &zone)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &name)...)
	if !known(zone) || !known(name) {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("fqdn"), fqdn(plan.name, plan.zone))...)
	for _, f := range r.spec.keyFields() {
		if !known(plan.fields[f.attr]) {
			return
		}
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), r.spec.id(plan))...)
}

func (r *recordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	plan, diags := r.spec.get(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rec, err := r.client.AddRecord(ctx, r.spec.addParams(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating "+r.spec.typ+" record", err.Error())
		return
	}

	if plan.disabled {
		// The add call has no "disable" param.
		rec, err = r.client.UpdateRecord(ctx, r.spec.updateParams(plan, plan))
		if err != nil {
			resp.Diagnostics.AddError("Error disabling "+r.spec.typ+" record", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(r.spec.set(ctx, &resp.State, r.spec.merge(plan, rec))...)
}

func (r *recordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state, diags := r.spec.get(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rec, found, err := r.find(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Error reading "+r.spec.typ+" record", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(r.spec.set(ctx, &resp.State, r.spec.merge(state, rec))...)
}

func (r *recordResource) find(ctx context.Context, v recordVals) (technitium.Record, bool, error) {
	records, err := r.client.GetRecords(ctx, strings.TrimSuffix(v.zone, "."), fqdn(v.name, v.zone), false)
	if technitium.IsZoneNotFound(err) {
		return technitium.Record{}, false, nil
	}
	if err != nil {
		return technitium.Record{}, false, err
	}
	for _, rec := range records {
		if strings.EqualFold(rec.Name, fqdn(v.name, v.zone)) && r.spec.matches(rec, v) {
			return rec, true, nil
		}
	}
	return technitium.Record{}, false, nil
}

func (r *recordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan, diags := r.spec.get(ctx, req.Plan)
	resp.Diagnostics.Append(diags...)
	state, diags := r.spec.get(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rec, err := r.client.UpdateRecord(ctx, r.spec.updateParams(state, plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating "+r.spec.typ+" record", err.Error())
		return
	}

	resp.Diagnostics.Append(r.spec.set(ctx, &resp.State, r.spec.merge(plan, rec))...)
}

func (r *recordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	state, diags := r.spec.get(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRecord(ctx, r.spec.deleteParams(state)); err != nil {
		resp.Diagnostics.AddError("Error deleting "+r.spec.typ+" record", err.Error())
	}
}

func (r *recordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	v, err := r.spec.parseID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	rec, found, err := r.find(ctx, v)
	if err != nil {
		resp.Diagnostics.AddError("Error importing "+r.spec.typ+" record", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Record not found", fmt.Sprintf("No %s record matches import ID %q.", r.spec.typ, req.ID))
		return
	}

	// Fields not in the ID start null and are filled from the server.
	for _, f := range r.spec.fields {
		if _, ok := v.fields[f.attr]; !ok {
			v.fields[f.attr] = nullValue(f.kind)
		}
	}
	resp.Diagnostics.Append(r.spec.set(ctx, &resp.State, r.spec.merge(v, rec))...)
}

func nullValue(k fieldKind) attr.Value {
	switch k {
	case kInt:
		return types.Int64Null()
	case kBool:
		return types.BoolNull()
	case kStringMap:
		return types.MapNull(types.StringType)
	default:
		return types.StringNull()
	}
}
