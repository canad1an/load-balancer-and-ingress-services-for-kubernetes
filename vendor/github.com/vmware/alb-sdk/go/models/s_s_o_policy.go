// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache License 2.0
package models

// This file is auto-generated.

// SSOPolicy s s o policy
// swagger:model SSOPolicy
type SSOPolicy struct {

	// UNIX time since epoch in microseconds. Units(MICROSECONDS).
	// Read Only: true
	LastModified *string `json:"_last_modified,omitempty"`

	// Authentication Policy Settings. Field introduced in 18.2.1.
	// Required: true
	AuthenticationPolicy *AuthenticationPolicy `json:"authentication_policy"`

	// Authorization Policy Settings. Field introduced in 18.2.5.
	AuthorizationPolicy *AuthorizationPolicy `json:"authorization_policy,omitempty"`

	// Key value pairs for granular object access control. Also allows for classification and tagging of similar objects. Field deprecated in 20.1.5. Field introduced in 20.1.2. Maximum of 4 items allowed.
	Labels []*KeyValue `json:"labels,omitempty"`

	// List of labels to be used for granular RBAC. Field introduced in 20.1.5. Allowed in Basic edition, Essentials edition, Enterprise edition.
	Markers []*RoleFilterMatchLabel `json:"markers,omitempty"`

	// Name of the SSO Policy. Field introduced in 18.2.3.
	// Required: true
	Name *string `json:"name"`

	// UUID of the Tenant. It is a reference to an object of type Tenant. Field introduced in 18.2.3.
	TenantRef *string `json:"tenant_ref,omitempty"`

	// SSO Policy Type. Enum options - SSO_TYPE_SAML, SSO_TYPE_PINGACCESS, SSO_TYPE_JWT. Field introduced in 18.2.5.
	// Required: true
	Type *string `json:"type"`

	// url
	// Read Only: true
	URL *string `json:"url,omitempty"`

	// UUID of the SSO Policy. Field introduced in 18.2.3.
	UUID *string `json:"uuid,omitempty"`
}
