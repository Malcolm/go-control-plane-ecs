# ECS Control Plane

This is an xDS control plane that uses AWS ECS metadata to dynamically update Envoy's endpoint configuration.

## Building

To build the `ecs-control-plane`, run the following command from the root of the repository:

```sh
go build ./cmd/ecs-control-plane
```

## Running

The `ecs-control-plane` requires AWS credentials to be configured in the environment where it's running. It uses the default AWS credential chain, so you can configure credentials using environment variables, IAM roles, or the `~/.aws/credentials` file.

### Flags

| Flag                 | Description                                | Default     |
| -------------------- | ------------------------------------------ | ----------- |
| `-port`              | xDS management server port                 | `18000`     |
| `-nodeID`            | Node ID to use for the snapshot cache      | `test-id`   |
| `-aws-region`        | AWS region for the ECS cluster             | `us-west-2` |
| `-ecs-cluster`       | Name of the ECS cluster to poll            | (required)  |
| `-ecs-service`       | Name of the ECS service to poll            | (required)  |
| `-polling-interval`  | Interval for polling ECS for new endpoints | `10s`       |
| `-debug`             | Enable debug logging                       | `false`     |

### Example

```sh
./ecs-control-plane \
    -aws-region us-east-1 \
    -ecs-cluster my-cluster \
    -ecs-service my-service
```

This will start the xDS server on port 18000 and begin polling the `my-service` service in the `my-cluster` ECS cluster in the `us-east-1` region for endpoint updates.
