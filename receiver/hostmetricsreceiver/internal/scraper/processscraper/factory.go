// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"context"
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

var (
	bootTimeCacheFeaturegateID = "hostmetrics.process.bootTimeCache"
	bootTimeCacheFeaturegate   = featuregate.GlobalRegistry().MustRegister(
		bootTimeCacheFeaturegateID,
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("When enabled, all process scrapes will use the boot time value that is cached at the start of the process."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/28849"),
		featuregate.WithRegisterFromVersion("v0.98.0"),
	)
)

// NewFactory for Process scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(metadata.Type, createDefaultConfig, scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the Scraper.
func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// createMetricsScraper creates a resource scraper based on provided config.
func createMetricsScraper(
	ctx context.Context,
	settings scraper.Settings,
	cfg component.Config,
) (scraper.Metrics, error) {
	if _, err := process.PidsWithContext(ctx); err != nil {
		return nil, fmt.Errorf("process scraper is not supported on %s: %w", runtime.GOOS, err)
	}

	s, err := newProcessScraper(settings, cfg.(*Config))
	if err != nil {
		return nil, err
	}

	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
	)
}
