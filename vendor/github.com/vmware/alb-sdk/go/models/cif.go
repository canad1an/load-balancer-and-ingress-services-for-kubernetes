// Copyright 2021 VMware, Inc.
// SPDX-License-Identifier: Apache License 2.0
package models

// This file is auto-generated.

// Cif cif
// swagger:model Cif
type Cif struct {

	// adapter of Cif.
	Adapter *string `json:"adapter,omitempty"`

	// cif of Cif.
	Cif *string `json:"cif,omitempty"`

	// mac_address of Cif.
	MacAddress *string `json:"mac_address,omitempty"`

	// resources of Cif.
	Resources []string `json:"resources,omitempty"`

	// Unique object identifier of se.
	SeUUID *string `json:"se_uuid,omitempty"`
}
