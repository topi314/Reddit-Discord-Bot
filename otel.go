package main

import (
	"net/http"

	"github.com/disgoorg/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/topi314/reddit-discord-bot/redditbot"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/semconv/v1.20.0"
)

func resources(cfg redditbot.OtelConfig) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(Name),
		semconv.ServiceNamespace(Namespace),
		semconv.ServiceInstanceID(cfg.InstanceID),
		semconv.ServiceVersion(Version),
	)
}

func newMeter(cfg redditbot.OtelConfig) (metric.Meter, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	exp, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exp),
		sdkmetric.WithResource(resources(cfg)),
	)
	otel.SetMeterProvider(mp)

	mux := http.NewServeMux()
	mux.Handle(cfg.Metrics.Endpoint, promhttp.Handler())
	server := &http.Server{
		Addr:    cfg.Metrics.ListenAddr,
		Handler: mux,
	}

	go func() {
		if listenErr := server.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			log.Error("failed to listen metrics server", err)
		}
	}()

	return mp.Meter(Name), nil
}
