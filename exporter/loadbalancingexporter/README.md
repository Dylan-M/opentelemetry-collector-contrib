# Trace ID/Service-name aware load-balancing exporter

<!-- status autogenerated section -->
| Status        |           |
| ------------- |-----------|
| Stability     | [development]: metrics   |
|               | [beta]: traces, logs   |
| Distributions | [contrib], [k8s] |
| Issues        | [![Open issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aopen%20label%3Aexporter%2Floadbalancing%20&label=open&color=orange&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aopen+is%3Aissue+label%3Aexporter%2Floadbalancing) [![Closed issues](https://img.shields.io/github/issues-search/open-telemetry/opentelemetry-collector-contrib?query=is%3Aissue%20is%3Aclosed%20label%3Aexporter%2Floadbalancing%20&label=closed&color=blue&logo=opentelemetry)](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aclosed+is%3Aissue+label%3Aexporter%2Floadbalancing) |
| Code coverage | [![codecov](https://codecov.io/github/open-telemetry/opentelemetry-collector-contrib/graph/main/badge.svg?component=exporter_loadbalancing)](https://app.codecov.io/gh/open-telemetry/opentelemetry-collector-contrib/tree/main/?components%5B0%5D=exporter_loadbalancing&displayType=list) |
| [Code Owners](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#becoming-a-code-owner)    | [@rlankfo](https://www.github.com/rlankfo) \| Seeking more code owners! |
| Emeritus      | [@jpkrohling](https://www.github.com/jpkrohling) |

[development]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#development
[beta]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#beta
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[k8s]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-k8s
<!-- end autogenerated section -->

This is an exporter that will consistently export spans, metrics and logs depending on the `routing_key` configured.

The options for `routing_key` are: `service`, `traceID`, `metric` (metric name), `resource`, `streamID`.

| routing_key | can be used for      |
| ----------- | -------------------- |
| service     | logs, spans, metrics |
| traceID     | logs, spans          |
| resource    | metrics              |
| metric      | metrics              |
| streamID    | metrics              |
| attributes  | spans                |

If no `routing_key` is configured, the default routing mechanism is `traceID`  for traces, while `service` is the default for metrics. This means that spans belonging to the same `traceID` (or `service.name`, when `service` is used as the `routing_key`) will be sent to the same backend.

It requires a source of backend information to be provided: static, with a fixed list of backends, or DNS, with a hostname that will resolve to all IP addresses to use (such as a Kubernetes headless service). The DNS resolver will periodically check for updates.

Note that either the Trace ID or Service name is used for the decision on which backend to use: the actual backend load isn't taken into consideration. Even though this load-balancer won't do round-robin balancing of the batches, the load distribution should be very similar among backends with a standard deviation under 5% at the current configuration.

This load balancer is especially useful for backends configured with tail-based samplers or red-metrics-collectors, which make a decision based on the view of the full trace.

When a list of backends is updated, some of the signals will be rerouted to different backends.
Around R/N of the "routes" will be rerouted differently, where:

* A "route" is either a trace ID or a service name mapped to a certain backend.
* "R" is the total number of routes.
* "N" is the total number of backends.

This should be stable enough for most cases, and the larger the number of backends, the less disruption it should cause. Still, if routing stability is important for your use case and your list of backends are constantly changing, consider using the `groupbytrace` processor. This way, traces are dispatched atomically to this exporter, and the same decision about the backend is made for the trace as a whole.

This also supports service name based exporting for traces. If you have two or more collectors that collect traces and then use spanmetrics connector to generate metrics and push to prometheus, there is a high chance of facing label collisions on prometheus if the routing is based on `traceID` because every collector sees the `service+operation` label. With service name based routing, each collector can only see one service name and can push metrics without any label collisions.

## Resilience and scaling considerations

The `loadbalancingexporter` will, irrespective of the chosen resolver (`static`, `dns`, `k8s`), create one `otlp` exporter per endpoint. Each level of exporters, `loadbalancingexporter` itself and all sub-exporters (one per each endpoint), have its own queue, timeout and retry mechanisms. Importantly, the `loadbalancingexporter`, by default, will NOT attempt to re-route data to a healthy endpoint on delivery failure, because in-memory queue, retry and timeout setting are disabled by default ([more details on queuing, retry and timeout default settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md)).

```
                                        +------------------+          +---------------+
 resiliency options 1                   |                  |          |               |
                                       -- otlp exporter 1  ------------  backend 1    |
           |                       ---/ |                  |          |               |
           |                   ---/     +----|-------------+          +---------------+
           |               ---/              |
  +-----------------+  ---/                  |
  |                 --/                      |
  |  loadbalancing  |                   resiliency options 2
  |    exporter     |                        |
  |                 --\                      |
  +-----------------+  ----\                 |
                            ----\       +----|-------------+          +---------------+
                                 ----\  |                  |          |               |
                                      --- otlp exporter N  ------------  backend N    |
                                        |                  |          |               |
                                        +------------------+          +---------------+
```

* For all types of resolvers (`static`, `dns`, `k8s`) - if one of endpoints is unavailable - first works queue, retry and timeout settings defined for sub-exporters (under `otlp` property). Once redelivery is exhausted on sub-exporter level, and resilience options 1 are enabled - telemetry data returns to `loadbalancingexporter` itself and data redelivery happens according to exporter level queue, retry and timeout settings.
* When using the `static` resolver and all targets are unavailable, all load-balanced telemetry will fail to be delivered until either one or all targets are restored or valid target is added the static list. The same principle applies to the `dns` and `k8s` resolvers, except for endpoints list update which happens automatically.
* When using `k8s`, `dns`, and likely future resolvers, topology changes are eventually reflected in the `loadbalancingexporter`. The `k8s` resolver will update more quickly than `dns`, but a window of time in which the true topology doesn't match the view of the `loadbalancingexporter` remains.
* Resiliency options 1 (`timeout`, `retry_on_failure` and `sending_queue` settings in `loadbalancing` section) - are useful for highly elastic environment (like k8s), where list of resolved endpoints frequently changed due to deployments, scale-up or scale-down events. In case of permanent change of list of resolved exporters this options provide capability to re-route data into new set of healthy backends. Disabled by default.
* Resiliency options 2 (`timeout`, `retry_on_failure` and `sending_queue` settings in `otlp` section) - are useful for temporary problems with specific backend, like network flukes. Persistent Queue is NOT supported here as all sub-exporter shares the same `sending_queue` configuration, including `storage`. Enabled by default.

Unfortunately, data loss is still possible if all of the exporter's targets remains unavailable once redelivery is exhausted. Due consideration needs to be given to the exporter queue and retry configuration when running in a highly elastic environment.

To avoid a single point of failure, requests can be distributed among multiple Collector instances configured with the `loadbalancingexporter`. The consistent hashing mechanism will ensure a deterministic result between instances sharing the same configuration and resolve an exact list of backend endpoints.

## Configuration

Refer to [config.yaml](./testdata/config.yaml) for detailed examples on using the exporter.

* The `otlp` property configures the template used for building the OTLP exporter. Refer to the OTLP Exporter documentation for information on which options are available. Note that the `endpoint` property should not be set and will be overridden by this exporter with the backend endpoint.
* The `resolver` accepts a `static` node, a `dns`, a `k8s` service or `aws_cloud_map`. If all four are specified, an `errMultipleResolversProvided` error will be thrown.
* The `hostname` property inside a `dns` node specifies the hostname to query in order to obtain the list of IP addresses.
* The `dns` node also accepts the following optional properties:
  * `hostname` DNS hostname to resolve.
  * `port` port to be used for exporting the traces to the IP addresses resolved from `hostname`. If `port` is not specified, the default port 4317 is used.
  * `interval` resolver interval in go-Duration format, e.g. `5s`, `1d`, `30m`. If not specified, `5s` will be used.
  * `timeout` resolver timeout in go-Duration format, e.g. `5s`, `1d`, `30m`. If not specified, `1s` will be used.
* The `k8s` node accepts the following optional properties:
  * `service` Kubernetes service to resolve, e.g. `lb-svc.lb-ns`. If no namespace is specified, an attempt will be made to infer the namespace for this collector, and if this fails it will fall back to the `default` namespace.
  * `ports` port to be used for exporting the traces to the addresses resolved from `service`. If `ports` is not specified, the default port 4317 is used. When multiple ports are specified, two backends are added to the load balancer as if they were at different pods.
  * `timeout` resolver timeout in go-Duration format, e.g. `5s`, `1d`, `30m`. If not specified, `1s` will be used.
  * `return_hostnames` will return hostnames instead of IPs. This is useful in certain situations like using istio in sidecar mode. To use this feature, the `service` must be a headless `Service`, pointing at a `StatefulSet`, and the `service` must be what is specified under `.spec.serviceName` in the `StatefulSet`.
* The `aws_cloud_map` node accepts the following properties:
  * `namespace` The CloudMap namespace where the service is register, e.g. `cloudmap`. If no `namespace` is specified, this will fail to start the Load Balancer exporter.
  * `service_name` The name of the service that you specified when you registered the instance, e.g. `otelcollectors`.  If no `service_name` is specified, this will fail to start the Load Balancer exporter.
  * `interval` resolver interval in go-Duration format, e.g. `5s`, `1d`, `30m`. If not specified, `30s` will be used.
  * `timeout` resolver timeout in go-Duration format, e.g. `5s`, `1d`, `30m`. If not specified, `5s` will be used.
  * `port` port to be used for exporting the traces to the addresses resolved from `service`. By default, the port is set in Cloud Map, but can be be overridden with a static value in this config
  * `health_status` filter in AWS Cloud Map, you can specify the health status of the instances that you want to discover. The health_status filter is optional and allows you to query based on the health status of the instances.
    * Available values are
      * `HEALTHY`: Only return instances that are healthy.
      * `UNHEALTHY`: Only return instances that are unhealthy.
      * `ALL`: Return all instances, regardless of their health status.
      * `HEALTHY_OR_ELSE_ALL`: Returns healthy instances, unless none are reporting a healthy state. In that case, return all instances. This is also called failing open.
    * Resolver's default filter is set to `HEALTHY` when none is explicitly defined
  * **Notes:**
    * This resolver currently returns a maximum of 100 hosts.
    * `TODO`: Feature request [29771](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/29771) aims to cover the pagination for this scenario
* The `routing_key` property is used to specify how to route values (spans or metrics) to exporters based on different parameters. This functionality is currently enabled only for `trace` and `metric` pipeline types. It supports one of the following values:
  * `service`: Routes values based on their service name. This is useful when using processors like the span metrics, so all spans for each service are sent to consistent collector instances for metric collection. Otherwise, metrics for the same services are sent to different collectors, making aggregations inaccurate.
  * `attributes`: Routes based on values in the attributes of the traces. This is similar to service, but useful for situations in which a single service overwhelms any given instance of the collector, and should be split over multiple collectors. In addition to resource / span attributes, `span.kind`, `span.name` (the top level properties of a span) are also supported.
  * `traceID`: Routes spans based on their `traceID`. Invalid for metrics.
  * `metric`: Routes metrics based on their metric name. Invalid for spans.
  * `streamID`: Routes metrics based on their datapoint streamID. That's the unique hash of all it's attributes, plus the attributes and identifying information of its resource, scope, and metric data
* loadbalancing exporter supports set of standard [queuing, retry and timeout settings](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md), but they are disable by default to maintain compatibility
* The `routing_attributes` property is used to list the attributes that should be used if the `routing_key` is `attributes`.

Simple example

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:

exporters:
  loadbalancing:
    routing_key: "service"
    protocol:
      otlp:
        # all options from the OTLP exporter are supported
        # except the endpoint
        timeout: 1s
    resolver:
      static:
        hostnames:
        - backend-1:4317
        - backend-2:4317
        - backend-3:4317
        - backend-4:4317
      # Notice to config a headless service DNS in Kubernetes
      # dns:
      #  hostname: otelcol-headless.observability.svc.cluster.local

service:
  pipelines:
    traces:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
    logs:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
```

Persistent queue, retry and timeout usage example:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:

exporters:
  loadbalancing:
    timeout: 10s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      num_consumers: 2
      queue_size: 1000
      storage: file_storage/otc
    routing_key: "service"
    protocol:
      otlp:
        # all options from the OTLP exporter are supported
        # except the endpoint
        timeout: 1s
        sending_queue:
          enabled: true
    resolver:
      static:
        hostnames:
        - backend-1:4317
        - backend-2:4317
        - backend-3:4317
        - backend-4:4317
      # Notice to config a headless service DNS in Kubernetes
      # dns:
      #  hostname: otelcol-headless.observability.svc.cluster.local

extensions:
  file_storage/otc:
    directory: /var/lib/storage/otc
    timeout: 10s

service:
  extensions: [file_storage]
  pipelines:
    traces:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
    logs:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
```

Kubernetes resolver example (For a more specific example: [example/k8s-resolver](./example/k8s-resolver/README.md))
> [!IMPORTANT]
> The k8s resolver requires proper permissions. See [the full example](./example/k8s-resolver/README.md) for more information.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:

exporters:
  loadbalancing:
    routing_key: "service"
    protocol:
      otlp:
        # all options from the OTLP exporter are supported
        # except the endpoint
        timeout: 1s
    resolver:
      # use k8s service resolver, if collector runs in kubernetes environment
      k8s:
        service: lb-svc.kube-public
        ports:
          - 15317
          - 16317

service:
  pipelines:
    traces:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
    logs:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
```

AWS CloudMap resolver example

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:

exporters:
  loadbalancing:
    protocol:
      otlp:
        # all options from the OTLP exporter are supported
        # except the endpoint
        timeout: 3s
    resolver:
      aws_cloud_map:
        namespace: aws-namespace
        service_name: aws-otel-col-service-name
        interval: 30s

service:
  pipelines:
    traces:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
    logs:
      receivers:
        - otlp
      processors: []
      exporters:
        - loadbalancing
```

For testing purposes, the following configuration can be used, where both the load balancer and all backends are running locally:

```yaml
receivers:
  otlp/loadbalancer:
    protocols:
      grpc:
        endpoint: localhost:4317
  otlp/backend-1:
    protocols:
      grpc:
        endpoint: localhost:55690
  otlp/backend-2:
    protocols:
      grpc:
        endpoint: localhost:55700
  otlp/backend-3:
    protocols:
      grpc:
        endpoint: localhost:55710
  otlp/backend-4:
    protocols:
      grpc:
        endpoint: localhost:55720

processors:

exporters:
  debug:
  loadbalancing:
    protocol:
      otlp:
        timeout: 1s
        tls:
          insecure: true
    resolver:
      static:
        hostnames:
        - localhost:55690
        - localhost:55700
        - localhost:55710
        - localhost:55720

service:
  pipelines:
    traces/loadbalancer:
      receivers:
        - otlp/loadbalancer
      processors: []
      exporters:
        - loadbalancing

    traces/backend-1:
      receivers:
        - otlp/backend-1
      processors: []
      exporters:
        - debug

    traces/backend-2:
      receivers:
        - otlp/backend-2
      processors: []
      exporters:
        - debug

    traces/backend-3:
      receivers:
        - otlp/backend-3
      processors: []
      exporters:
        - debug

    traces/backend-4:
      receivers:
        - otlp/backend-4
      processors: []
      exporters:
        - debug

    logs/loadbalancer:
      receivers:
        - otlp/loadbalancer
      processors: []
      exporters:
        - loadbalancing
    logs/backend-1:
      receivers:
        - otlp/backend-1
      processors: []
      exporters:
        - debug
    logs/backend-2:
      receivers:
        - otlp/backend-2
      processors: []
      exporters:
        - debug
    logs/backend-3:
      receivers:
        - otlp/backend-3
      processors: []
      exporters:
        - debug
    logs/backend-4:
      receivers:
        - otlp/backend-4
      processors: []
      exporters:
        - debug
```

## Metrics

The following metrics are recorded by this exporter:

* `otelcol_loadbalancer_num_resolutions` represents the total number of resolutions performed by the resolver specified in the tag `resolver`, split by their outcome (`success=true|false`). For the static resolver, this should always be `1` with the tag `success=true`.
* `otelcol_loadbalancer_num_backends` informs how many backends are currently in use. It should always match the number of items specified in the configuration file in case the `static` resolver is used, and should eventually (seconds) catch up with the DNS changes. Note that DNS caches that might exist between the load balancer and the record authority will influence how long it takes for the load balancer to see the change.
* `otelcol_loadbalancer_num_backend_updates` records how many of the resolutions resulted in a new list of backends. Use this information to understand how frequent your backend updates are and how often the ring is rebalanced. If the DNS hostname is always returning the same list of IP addresses but this metric keeps increasing, it might indicate a bug in the load balancer.
* `otelcol_loadbalancer_backend_latency` measures the latency for each backend.
* `otelcol_loadbalancer_backend_outcome` counts what the outcomes were for each endpoint, `success=true|false`.
