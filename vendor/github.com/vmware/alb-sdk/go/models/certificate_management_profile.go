// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache License 2.0
package models

// This file is auto-generated.

// CertificateManagementProfile certificate management profile
// swagger:model CertificateManagementProfile
type CertificateManagementProfile struct {

	// UNIX time since epoch in microseconds. Units(MICROSECONDS).
	// Read Only: true
	LastModified *string `json:"_last_modified,omitempty"`

	// List of labels to be used for granular RBAC. Field introduced in 20.1.6. Allowed in Basic edition, Essentials edition, Enterprise edition.
	Markers []*RoleFilterMatchLabel `json:"markers,omitempty"`

	// Name of the PKI Profile.
	// Required: true
	Name *string `json:"name"`

	// Alert script config object for certificate management profile. It is a reference to an object of type AlertScriptConfig. Field introduced in 20.1.3.
	// Required: true
	RunScriptRef *string `json:"run_script_ref"`

	// Placeholder for description of property script_params of obj type CertificateManagementProfile field type str  type object
	ScriptParams []*CustomParams `json:"script_params,omitempty"`

	//  Field deprecated in 20.1.3.
	ScriptPath *string `json:"script_path,omitempty"`

	//  It is a reference to an object of type Tenant.
	TenantRef *string `json:"tenant_ref,omitempty"`

	// url
	// Read Only: true
	URL *string `json:"url,omitempty"`

	// Unique object identifier of the object.
	UUID *string `json:"uuid,omitempty"`
}
