// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache License 2.0
package models

// This file is auto-generated.

// VrfContext vrf context
// swagger:model VrfContext
type VrfContext struct {

	// UNIX time since epoch in microseconds. Units(MICROSECONDS).
	// Read Only: true
	LastModified *string `json:"_last_modified,omitempty"`

	// Key/value vrfcontext attributes. Field introduced in 20.1.2. Allowed in Basic edition, Essentials edition, Enterprise edition.
	Attrs []*KeyValue `json:"attrs,omitempty"`

	// BFD configuration profile. Field introduced in 20.1.1. Allowed in Basic edition, Essentials edition, Enterprise edition.
	BfdProfile *BfdProfile `json:"bfd_profile,omitempty"`

	// Bgp Local and Peer Info.
	BgpProfile *BgpProfile `json:"bgp_profile,omitempty"`

	//  It is a reference to an object of type Cloud.
	CloudRef *string `json:"cloud_ref,omitempty"`

	// Configure debug flags for VRF. Field introduced in 17.1.1.
	Debugvrfcontext *DebugVrfContext `json:"debugvrfcontext,omitempty"`

	// User defined description for the object.
	Description *string `json:"description,omitempty"`

	// Configure ping based heartbeat check for gateway in service engines of vrf. Maximum of 32 items allowed.
	GatewayMon []*GatewayMonitor `json:"gateway_mon,omitempty"`

	// Configure ping based heartbeat check for all default gateways in service engines of vrf. Field introduced in 17.1.1.
	InternalGatewayMonitor *InternalGatewayMonitor `json:"internal_gateway_monitor,omitempty"`

	// Key/value labels which can be used for Object Access Policy permission scoping. Field deprecated in 20.1.5. Field introduced in 18.2.7, 20.1.1.
	Labels []*KeyValue `json:"labels,omitempty"`

	// Enable LLDP. Field introduced in 18.2.10, 20.1.1. Allowed in Basic(Allowed values- true) edition, Essentials(Allowed values- true) edition, Enterprise edition.
	LldpEnable *bool `json:"lldp_enable,omitempty"`

	// List of labels to be used for granular RBAC. Field introduced in 20.1.5. Allowed in Basic edition, Essentials edition, Enterprise edition.
	Markers []*RoleFilterMatchLabel `json:"markers,omitempty"`

	// Name of the object.
	// Required: true
	Name *string `json:"name"`

	// Placeholder for description of property static_routes of obj type VrfContext field type str  type object
	StaticRoutes []*StaticRoute `json:"static_routes,omitempty"`

	// Placeholder for description of property system_default of obj type VrfContext field type str  type boolean
	SystemDefault *bool `json:"system_default,omitempty"`

	//  It is a reference to an object of type Tenant.
	TenantRef *string `json:"tenant_ref,omitempty"`

	// url
	// Read Only: true
	URL *string `json:"url,omitempty"`

	// Unique object identifier of the object.
	UUID *string `json:"uuid,omitempty"`
}
