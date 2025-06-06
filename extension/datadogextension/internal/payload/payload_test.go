// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package payload // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/payload"

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitPayloadInterface(t *testing.T) {
	payload := &OtelCollectorPayload{}
	_, err := payload.SplitPayload(1)
	assert.Error(t, err)
	assert.ErrorContains(t, err, payloadSplitErr)
}

func TestPrepareOtelCollectorPayload(t *testing.T) {
	hostname := "test-hostname"
	hostnameSource := "config"
	extensionUUID := "test-uuid"
	version := "1.0.0"
	site := "datadoghq.com"
	fullConfig := "{\"service\":{\"pipelines\":{\"traces\":{\"receivers\":[\"otlp\"],\"exporters\":[\"debug\"]}}}}"
	buildInfo := CustomBuildInfo{
		Command: "otelcol",
		Version: "1.0.0",
	}

	expectedPayload := OtelCollector{
		HostKey:           "",
		Hostname:          hostname,
		HostnameSource:    hostnameSource,
		CollectorID:       hostname + "-" + extensionUUID,
		CollectorVersion:  version,
		ConfigSite:        site,
		APIKeyUUID:        "",
		BuildInfo:         buildInfo,
		FullConfiguration: fullConfig,
	}

	actualPayload := PrepareOtelCollectorPayload(hostname, hostnameSource, extensionUUID, version, site, fullConfig, buildInfo)

	assert.Equal(t, expectedPayload, actualPayload)
}

func TestOtelCollectorPayload_MarshalJSON(t *testing.T) {
	oc := &OtelCollectorPayload{
		Hostname:  "test_host",
		Timestamp: time.Now().UnixNano(),
		UUID:      "test-uuid",
		Metadata: OtelCollector{
			FullComponents: []CollectorModule{
				{
					Type:       "test_type",
					Kind:       "test_kind",
					Gomod:      "test_gomod",
					Version:    "test_version",
					Configured: true,
				},
			},
		},
	}

	jsonData, err := json.Marshal(oc)
	require.NoError(t, err)

	var unmarshaled OtelCollectorPayload
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, oc.Hostname, unmarshaled.Hostname)
	assert.Equal(t, oc.UUID, unmarshaled.UUID)
	assert.Equal(t, oc.Metadata.FullComponents, unmarshaled.Metadata.FullComponents)
}

func TestOtelCollectorPayload_UnmarshalAndMarshal(t *testing.T) {
	// Read the sample JSON payload from file
	filePath := "testdata/sample-otelcollectorpayload.json"
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Unmarshal the JSON into an OtelCollectorPayload struct
	var payload OtelCollectorPayload
	err = json.Unmarshal(data, &payload)
	require.NoError(t, err)

	// Marshal the struct back into JSON
	marshaledJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	// Unmarshal the marshaled JSON back into a struct
	var unmarshaledPayload OtelCollectorPayload
	err = json.Unmarshal(marshaledJSON, &unmarshaledPayload)
	require.NoError(t, err)

	// Assert that the original and unmarshaled structs are equal
	assert.Equal(t, payload, unmarshaledPayload)
}
