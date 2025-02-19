// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache License 2.0
package models

// This file is auto-generated.

// VinfraVcenterBadCredentials vinfra vcenter bad credentials
// swagger:model VinfraVcenterBadCredentials
type VinfraVcenterBadCredentials struct {

	// Name of the object.
	Name *string `json:"name,omitempty"`

	// Number of previous_count.
	PreviousCount *int64 `json:"previous_count,omitempty"`

	// user of VinfraVcenterBadCredentials.
	User *string `json:"user,omitempty"`

	// vcenter of VinfraVcenterBadCredentials.
	// Required: true
	Vcenter *string `json:"vcenter"`

	// vcenter_name of VinfraVcenterBadCredentials.
	VcenterName *string `json:"vcenter_name,omitempty"`

	// vcenter_object of VinfraVcenterBadCredentials.
	VcenterObject *string `json:"vcenter_object,omitempty"`
}
