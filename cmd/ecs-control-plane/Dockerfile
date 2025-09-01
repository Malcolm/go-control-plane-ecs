# ---- Builder Stage ----
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# CGO_ENABLED=0 is important for a static binary with scratch
# -ldflags="-s -w" strips debug symbols to make the binary smaller
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /ecs-control-plane .

# ---- Final Stage ----
FROM scratch

# Copy the compiled binary from the builder stage
COPY --from=builder /ecs-control-plane /ecs-control-plane

# Expose the default ports (optional, but good practice)
# 18000 for xDS, 9090 for metrics
EXPOSE 18000
EXPOSE 9090

# Set the entrypoint for the container
ENTRYPOINT ["/ecs-control-plane"]
