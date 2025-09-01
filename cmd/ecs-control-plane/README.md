# ECS Control Plane

This is a production-grade xDS control plane that uses AWS ECS metadata to dynamically update Envoy's endpoint configuration.

## Features

-   Dynamic endpoint discovery from AWS ECS.
-   Structured logging with `zap`.
-   Prometheus metrics for observability.
-   Health check endpoint.
-   Graceful shutdown.
-   Advanced configuration via file, environment variables, and flags.

## Building

To build the `ecs-control-plane`, run the following command from the root of the repository:

```sh
go build ./cmd/ecs-control-plane
```

## Configuration

The application can be configured via a YAML file, environment variables, or command-line flags. The order of precedence is: Flags > Environment Variables > Config File > Defaults.

### Config File

Create a `config.yaml` file in the same directory as the binary, or in `/etc/ecs-control-plane/`.

Example `config.yaml`:
```yaml
debug: true
port: 18000
metrics-port: 9090
node-id: "my-node"
aws-region: "us-east-1"
ecs-cluster: "my-production-cluster"
ecs-service: "my-web-service"
polling-interval: "15s"
```

### Environment Variables

All configuration options can be set with environment variables, prefixed with `XDS_`. Use underscores instead of hyphens.

Example:
```sh
export XDS_AWS_REGION="us-east-1"
export XDS_ECS_CLUSTER="my-cluster"
export XDS_ECS_SERVICE="my-service"
./ecs-control-plane
```

### Command-line Flags

| Flag                 | Config Key           | Environment Variable      | Default     | Description                                |
| -------------------- | -------------------- | ------------------------- | ----------- | ------------------------------------------ |
| `-debug`             | `debug`              | `XDS_DEBUG`               | `false`     | Enable debug logging.                      |
| `-port`              | `port`               | `XDS_PORT`                | `18000`     | xDS management server port.                |
| `-metrics-port`      | `metrics-port`       | `XDS_METRICS_PORT`        | `9090`      | Port for the metrics and health server.    |
| `-node-id`           | `node-id`            | `XDS_NODE_ID`             | `test-id`   | Node ID to use for the snapshot cache.     |
| `-aws-region`        | `aws-region`         | `XDS_AWS_REGION`          | `us-west-2` | AWS region for the ECS cluster.            |
| `-ecs-cluster`       | `ecs-cluster`        | `XDS_ECS_CLUSTER`         | (required)  | Name of the ECS cluster to poll.           |
| `-ecs-service`       | `ecs-service`        | `XDS_ECS_SERVICE`         | (required)  | Name of the ECS service to poll.           |
| `-polling-interval`  | `polling-interval`   | `XDS_POLLING_INTERVAL`    | `10s`       | Interval for polling ECS for new endpoints.|

## Observability

### Health Checks

A health check endpoint is available at `/healthz` on the metrics port (default `9090`).

```sh
curl http://localhost:9090/healthz
```

### Prometheus Metrics

Prometheus metrics are exposed at `/metrics` on the metrics port (default `9090`). Key metrics include:
-   `xds_endpoints_discovered`: The current number of endpoints discovered from ECS.
-   `xds_snapshot_updates_total`: The total number of snapshot updates.
-   `xds_aws_api_errors_total`: The total number of errors from the AWS API.
