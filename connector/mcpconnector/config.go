// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package mcpconnector

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the MCP connector
type Config struct {
	// No configuration needed - the connector automatically finds the MCP extension
}

var _ component.Config = (*Config)(nil)

// Validate checks if the connector configuration is valid
func (*Config) Validate() error {
	return nil
}
