# ECS Control Plane

This is an xDS control plane that uses AWS ECS metadata to dynamically update Envoy's endpoint configuration.

## Building

To build the `ecs-control-plane`, run the following command from the root of the repository:

```sh
go build ./cmd/ecs-control-plane
```

## Configuration

The application can be configured via environment variables or command-line flags. The order of precedence is: Flags > Environment Variables > Defaults.

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

| Flag                 | Environment Variable      | Default     | Description                                |
| -------------------- | ------------------------- | ----------- | ------------------------------------------ |
| `-port`              | `XDS_PORT`                | `18000`     | xDS management server port.                |
| `-node-id`           | `XDS_NODE_ID`             | `test-id`   | Node ID to use for the snapshot cache.     |
| `-aws-region`        | `XDS_AWS_REGION`          | `us-west-2` | AWS region for the ECS cluster.            |
| `-ecs-cluster`       | `XDS_ECS_CLUSTER`         | (required)  | Name of the ECS cluster to poll.           |
| `-ecs-service`       | `XDS_ECS_SERVICE`         | (required)  | Name of the ECS service to poll.           |
| `-polling-interval`  | `XDS_POLLING_INTERVAL`    | `10s`       | Interval for polling ECS for new endpoints.|
