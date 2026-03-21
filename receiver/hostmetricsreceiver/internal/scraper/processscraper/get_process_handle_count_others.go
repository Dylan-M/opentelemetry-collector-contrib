// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const handleCountMetricsLen = 0

func (*wrappedProcessHandle) GetProcessHandleCountWithContext(context.Context) (int64, error) {
	return 0, nil
}

func (s *processScraper) scrapeAndAppendHandlesMetric(_ context.Context, _ pcommon.Timestamp, _ processHandle) error {
	return nil
}
