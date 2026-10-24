package provider

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"gitlab.com/stellivix/terraform-provider-technitium/internal/technitium"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &dnssecResource{}
	_ resource.ResourceWithImportState = &dnssecResource{}
)

func NewDnssecResource() resource.Resource { return &dnssecResource{} }

type dnssecResource struct {
	client *technitium.Client
}

type dnssecModel struct {
	ID              types.String `tfsdk:"id"`
	Zone            types.String `tfsdk:"zone"`
	Algorithm       types.String `tfsdk:"algorithm"`
	Curve           types.String `tfsdk:"curve"`
	HashAlgorithm   types.String `tfsdk:"hash_algorithm"`
	KskKeySize      types.Int64  `tfsdk:"ksk_key_size"`
	ZskKeySize      types.Int64  `tfsdk:"zsk_key_size"`
	NxProof         types.String `tfsdk:"nx_proof"`
	Nsec3Iterations types.Int64  `tfsdk:"nsec3_iterations"`
	Nsec3SaltLength types.Int64  `tfsdk:"nsec3_salt_length"`
	DnsKeyTtl       types.Int64  `tfsdk:"dnskey_ttl"`
	ZskRolloverDays types.Int64  `tfsdk:"zsk_rollover_days"`
	Status          types.String `tfsdk:"status"`
	Keys            types.List   `tfsdk:"keys"`
	DsRecords       types.List   `tfsdk:"ds_records"`
}

var (
	dnssecKeyType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"key_tag": types.Int64Type, "key_type": types.StringType, "algorithm": types.StringType,
		"state": types.StringType, "retiring": types.BoolType, "rollover_days": types.Int64Type,
	}}
	dsRecordType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"key_tag": types.Int64Type, "algorithm": types.StringType, "digest_type": types.StringType,
		"digest": types.StringType, "state": types.StringType,
	}}
)

func (r *dnssecResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone_dnssec"
}

func (r *dnssecResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = ConfigureResourceClient(req, resp)
}

func (r *dnssecResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceInt := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Signs a primary zone with DNSSEC. Destroying it unsigns the zone. Changing a signing parameter " +
			"(algorithm, curve, key sizes) re-signs with new keys, which requires updating the DS record at the parent; " +
			"publish `ds_records` there. Key rollover is automatic on the server.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, Description: "Zone name.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"zone": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "Primary zone to sign."},
			"algorithm": schema.StringAttribute{Required: true, PlanModifiers: replace, Description: "`ECDSA`, `EDDSA` or `RSA`.",
				Validators: []validator.String{stringvalidator.OneOf("ECDSA", "EDDSA", "RSA")}},
			"curve": schema.StringAttribute{Optional: true, PlanModifiers: replace,
				Description: "`P256` or `P384` for ECDSA; `ED25519` or `ED448` for EDDSA."},
			"hash_algorithm": schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "RSA only: `SHA256` or `SHA512` (or legacy `SHA1`, `MD5`)."},
			"ksk_key_size":   schema.Int64Attribute{Optional: true, PlanModifiers: replaceInt, Description: "RSA only: KSK size in bits."},
			"zsk_key_size":   schema.Int64Attribute{Optional: true, PlanModifiers: replaceInt, Description: "RSA only: ZSK size in bits."},
			"nx_proof": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("NSEC"),
				Description: "Proof of non-existence: `NSEC` or `NSEC3`. Changed in place.",
				Validators:  []validator.String{stringvalidator.OneOf("NSEC", "NSEC3")}},
			"nsec3_iterations": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				Description: "NSEC3 hash iterations (RFC 9276 recommends 0)."},
			"nsec3_salt_length": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0),
				Description: "NSEC3 salt length in bytes (RFC 9276 recommends 0)."},
			"dnskey_ttl": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(86400),
				Description: "TTL of the DNSKEY RRset. Set at signing; afterwards it can only be changed once every key is " +
					"`Ready` or `Active`, which for a new KSK takes several hours."},
			"zsk_rollover_days": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(30),
				Description: "Automatic ZSK rollover period in days; 0 disables rollover. Applied to all current ZSKs."},
			"status": schema.StringAttribute{Computed: true, Description: "DNSSEC status, e.g. `SignedWithNSEC3`."},
			"keys": schema.ListAttribute{Computed: true, ElementType: dnssecKeyType,
				Description: "Private keys: `key_tag`, `key_type`, `algorithm`, `state`, `retiring`, `rollover_days`."},
			"ds_records": schema.ListAttribute{Computed: true, ElementType: dsRecordType,
				Description: "DS records to publish at the parent, one per KSK and digest: `key_tag`, `algorithm`, `digest_type`, `digest`, `state`."},
		},
	}
}

func (r *dnssecResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnssecModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := url.Values{}
	p.Set("algorithm", plan.Algorithm.ValueString())
	addIfSet(p, "curve", plan.Curve.ValueString())
	addIfSet(p, "hashAlgorithm", plan.HashAlgorithm.ValueString())
	if known(plan.KskKeySize) {
		p.Set("kskKeySize", strconv.FormatInt(plan.KskKeySize.ValueInt64(), 10))
	}
	if known(plan.ZskKeySize) {
		p.Set("zskKeySize", strconv.FormatInt(plan.ZskKeySize.ValueInt64(), 10))
	}
	p.Set("nxProof", plan.NxProof.ValueString())
	p.Set("iterations", strconv.FormatInt(plan.Nsec3Iterations.ValueInt64(), 10))
	p.Set("saltLength", strconv.FormatInt(plan.Nsec3SaltLength.ValueInt64(), 10))
	p.Set("dnsKeyTtl", strconv.FormatInt(plan.DnsKeyTtl.ValueInt64(), 10))
	p.Set("zskRolloverDays", strconv.FormatInt(plan.ZskRolloverDays.ValueInt64(), 10))

	if err := r.client.SignZone(ctx, plan.Zone.ValueString(), p); err != nil {
		resp.Diagnostics.AddError("Error signing zone", err.Error())
		return
	}
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dnssecResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnssecModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	props, err := r.client.GetDnssecProperties(ctx, state.Zone.ValueString())
	if technitium.IsZoneNotFound(err) || (err == nil && props.DnssecStatus == "Unsigned") {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNSSEC properties", err.Error())
		return
	}
	resp.Diagnostics.Append(r.refresh(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dnssecResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dnssecModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zone := plan.Zone.ValueString()
	fail := func(what string, err error) bool {
		if err != nil {
			resp.Diagnostics.AddError("Error updating DNSSEC "+what, err.Error())
			return true
		}
		return false
	}

	if !plan.NxProof.Equal(state.NxProof) && fail("proof type", r.client.SetNxProof(ctx, zone, plan.NxProof.ValueString())) {
		return
	}
	if plan.NxProof.ValueString() == "NSEC3" &&
		(!plan.Nsec3Iterations.Equal(state.Nsec3Iterations) || !plan.Nsec3SaltLength.Equal(state.Nsec3SaltLength) || !plan.NxProof.Equal(state.NxProof)) &&
		fail("NSEC3 parameters", r.client.UpdateNsec3Params(ctx, zone, plan.Nsec3Iterations.ValueInt64(), plan.Nsec3SaltLength.ValueInt64())) {
		return
	}
	if !plan.DnsKeyTtl.Equal(state.DnsKeyTtl) && fail("DNSKEY TTL", r.client.UpdateDnsKeyTtl(ctx, zone, plan.DnsKeyTtl.ValueInt64())) {
		return
	}
	if !plan.ZskRolloverDays.Equal(state.ZskRolloverDays) {
		props, err := r.client.GetDnssecProperties(ctx, zone)
		if fail("keys", err) {
			return
		}
		for _, k := range props.PrivateKeys {
			if k.KeyType == "ZoneSigningKey" && !k.IsRetiring &&
				fail("ZSK rollover", r.client.UpdatePrivateKeyRollover(ctx, zone, k.KeyTag, plan.ZskRolloverDays.ValueInt64())) {
				return
			}
		}
	}
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dnssecResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnssecModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UnsignZone(ctx, state.Zone.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error unsigning zone", err.Error())
	}
}

// ImportState takes "<zone>" or "<zone>/<algorithm>[/<curve>]"; signing
// parameters other than the algorithm can't be read back exactly.
func (r *dnssecResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone"), parts[0])...)
	if len(parts) > 1 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("algorithm"), parts[1])...)
	}
	if len(parts) > 2 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("curve"), parts[2])...)
	}
}

func (r *dnssecResource) refresh(ctx context.Context, m *dnssecModel) diag.Diagnostics {
	var diags diag.Diagnostics
	zone := m.Zone.ValueString()
	props, err := r.client.GetDnssecProperties(ctx, zone)
	if err != nil {
		diags.AddError("Error reading DNSSEC properties", err.Error())
		return diags
	}
	ds, err := r.client.GetDsRecords(ctx, zone)
	if err != nil {
		diags.AddError("Error reading DS records", err.Error())
		return diags
	}

	m.ID = types.StringValue(zone)
	m.Status = types.StringValue(props.DnssecStatus)
	m.DnsKeyTtl = types.Int64Value(props.DnsKeyTtl)
	if strings.Contains(props.DnssecStatus, "NSEC3") {
		m.NxProof = types.StringValue("NSEC3")
	} else {
		m.NxProof = types.StringValue("NSEC")
	}
	if props.Nsec3Iterations != nil {
		m.Nsec3Iterations = types.Int64Value(*props.Nsec3Iterations)
	}
	if props.Nsec3SaltLength != nil {
		m.Nsec3SaltLength = types.Int64Value(*props.Nsec3SaltLength)
	}
	// Not returned while using NSEC; keep the configured value or default.
	if !known(m.Nsec3Iterations) {
		m.Nsec3Iterations = types.Int64Value(0)
	}
	if !known(m.Nsec3SaltLength) {
		m.Nsec3SaltLength = types.Int64Value(0)
	}
	if !known(m.Algorithm) {
		// After import: derive the family from the key algorithm name.
		for _, k := range props.PrivateKeys {
			switch {
			case strings.HasPrefix(k.Algorithm, "ECDSA"):
				m.Algorithm = types.StringValue("ECDSA")
			case strings.HasPrefix(k.Algorithm, "ED"):
				m.Algorithm = types.StringValue("EDDSA")
			case strings.HasPrefix(k.Algorithm, "RSA"):
				m.Algorithm = types.StringValue("RSA")
			}
		}
	}
	for _, k := range props.PrivateKeys {
		if k.KeyType == "ZoneSigningKey" && !k.IsRetiring {
			m.ZskRolloverDays = types.Int64Value(k.RolloverDays)
			break
		}
	}

	m.Keys = objectList(&diags, dnssecKeyType, len(props.PrivateKeys), func(i int) map[string]attr.Value {
		k := props.PrivateKeys[i]
		return map[string]attr.Value{
			"key_tag": types.Int64Value(k.KeyTag), "key_type": types.StringValue(k.KeyType), "algorithm": types.StringValue(k.Algorithm),
			"state": types.StringValue(k.State), "retiring": types.BoolValue(k.IsRetiring), "rollover_days": types.Int64Value(k.RolloverDays),
		}
	})
	type dsRow struct {
		tag                         int64
		alg, digestType, digest, st string
	}
	var rows []dsRow
	for _, d := range ds {
		for _, dg := range d.Digests {
			rows = append(rows, dsRow{d.KeyTag, d.Algorithm, dg.DigestType, dg.Digest, d.State})
		}
	}
	m.DsRecords = objectList(&diags, dsRecordType, len(rows), func(i int) map[string]attr.Value {
		x := rows[i]
		return map[string]attr.Value{
			"key_tag": types.Int64Value(x.tag), "algorithm": types.StringValue(x.alg), "digest_type": types.StringValue(x.digestType),
			"digest": types.StringValue(x.digest), "state": types.StringValue(x.st),
		}
	})
	return diags
}
