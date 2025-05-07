# MKP - Model Kontext Protocol Server for Kubernetes

<p align="center">
  <img src="docs/assets/mkp-logo.png" width="400" alt="MKP Logo">
</p>

MKP is a Model Context Protocol (MCP) server for Kubernetes that allows LLM-powered applications to interact with Kubernetes clusters. It provides tools for listing and applying Kubernetes resources through the MCP protocol.

## Features

- List resources supported by the Kubernetes API server
- List clustered resources
- List namespaced resources
- Get resources and their subresources (including status, scale, logs, etc.)
- Apply (create or update) clustered resources
- Apply (create or update) namespaced resources
- Generic and pluggable implementation using API Machinery's unstructured client
- Built-in rate limiting for protection against excessive API calls

## Why MKP?

MKP offers several key advantages as a Model Context Protocol server for Kubernetes:

### Native Go Implementation
- Built with the same language as Kubernetes itself
- Excellent performance characteristics for server applications
- Strong type safety and concurrency support
- Seamless integration with Kubernetes libraries

### Direct API Integration
- Uses Kubernetes API machinery directly without external dependencies
- No reliance on kubectl, helm, or other CLI tools
- Communicates directly with the Kubernetes API server
- Reduced overhead and improved reliability

### Universal Resource Support
- Works with any Kubernetes resource type through the unstructured client
- No hardcoded resource schemas or specialized handlers needed
- Automatically supports Custom Resource Definitions (CRDs)
- Future-proof for new Kubernetes resources

### Minimalist Design
- Focused on core Kubernetes resource operations
- Clean, maintainable codebase with clear separation of concerns
- Lightweight with minimal dependencies
- Easy to understand, extend, and contribute to

### Production-Ready Architecture
- Designed for reliability and performance in production environments
- Proper error handling and resource management
- Built-in rate limiting to protect against excessive API calls
- Testable design with comprehensive unit tests
- Follows Kubernetes development best practices

## Prerequisites

- Go 1.24 or later
- Kubernetes cluster and kubeconfig
- [Task](https://taskfile.dev/) for running tasks

## Installation

1. Clone the repository:

```bash
git clone https://github.com/StacklokLabs/mkp.git
cd mkp
```

2. Install dependencies:

```bash
task install
```

3. Build the server:

```bash
task build
```

## Usage

### Running the server

To run the server with the default kubeconfig:

```bash
task run
```

To run the server with a specific kubeconfig:

```bash
KUBECONFIG=/path/to/kubeconfig task run-with-kubeconfig
```

### MCP Tools

The MKP server provides the following MCP tools:

#### get_resource

Get a Kubernetes resource or its subresource.

Parameters:
- `resource_type` (required): Type of resource to get (clustered or namespaced)
- `group`: API group (e.g., apps, networking.k8s.io)
- `version` (required): API version (e.g., v1, v1beta1)
- `resource` (required): Resource name (e.g., deployments, services)
- `namespace`: Namespace (required for namespaced resources)
- `name` (required): Name of the resource to get
- `subresource`: Subresource to get (e.g., status, scale, logs)
- `parameters`: Optional parameters for the request (see examples below)

Example:

```json
{
  "name": "get_resource",
  "arguments": {
    "resource_type": "namespaced",
    "group": "apps",
    "version": "v1",
    "resource": "deployments",
    "namespace": "default",
    "name": "nginx-deployment",
    "subresource": "status"
  }
}
```

Example of getting logs from a specific container with parameters:

```json
{
  "name": "get_resource",
  "arguments": {
    "resource_type": "namespaced",
    "group": "",
    "version": "v1",
    "resource": "pods",
    "namespace": "default",
    "name": "my-pod",
    "subresource": "logs",
    "parameters": {
      "container": "my-container",
      "sinceSeconds": "3600",
      "timestamps": "true",
      "limitBytes": "102400"
    }
  }
}
```

Available parameters for pod logs:
- `container`: Specify which container to get logs from
- `previous`: Get logs from previous container instance (true/false)
- `sinceSeconds`: Only return logs newer than a relative duration in seconds
- `sinceTime`: Only return logs after a specific time (RFC3339 format)
- `timestamps`: Include timestamps on each line (true/false)
- `limitBytes`: Maximum number of bytes to return
- `tailLines`: Number of lines to return from the end of the logs

By default, pod logs are limited to the last 100 lines and 32KB to avoid overwhelming the LLM's context window. These defaults can be overridden using the parameters above.

Available parameters for regular resources:
- `resourceVersion`: When specified, shows the resource at that particular version

#### list_resources

Lists Kubernetes resources of a specific type.

Parameters:
- `resource_type` (required): Type of resource to list (clustered or namespaced)
- `group`: API group (e.g., apps, networking.k8s.io)
- `version` (required): API version (e.g., v1, v1beta1)
- `resource` (required): Resource name (e.g., deployments, services)
- `namespace`: Namespace (required for namespaced resources)

Example:

```json
{
  "name": "list_resources",
  "arguments": {
    "resource_type": "namespaced",
    "group": "apps",
    "version": "v1",
    "resource": "deployments",
    "namespace": "default"
  }
}
```

#### apply_resource

Applies (creates or updates) a Kubernetes resource.

Parameters:
- `resource_type` (required): Type of resource to apply (clustered or namespaced)
- `group`: API group (e.g., apps, networking.k8s.io)
- `version` (required): API version (e.g., v1, v1beta1)
- `resource` (required): Resource name (e.g., deployments, services)
- `namespace`: Namespace (required for namespaced resources)
- `manifest` (required): Resource manifest

Example:

```json
{
  "name": "apply_resource",
  "arguments": {
    "resource_type": "namespaced",
    "group": "apps",
    "version": "v1",
    "resource": "deployments",
    "namespace": "default",
    "manifest": {
      "apiVersion": "apps/v1",
      "kind": "Deployment",
      "metadata": {
        "name": "nginx-deployment",
        "namespace": "default"
      },
      "spec": {
        "replicas": 3,
        "selector": {
          "matchLabels": {
            "app": "nginx"
          }
        },
        "template": {
          "metadata": {
            "labels": {
              "app": "nginx"
            }
          },
          "spec": {
            "containers": [
              {
                "name": "nginx",
                "image": "nginx:latest",
                "ports": [
                  {
                    "containerPort": 80
                  }
                ]
              }
            ]
          }
        }
      }
    }
  }
}
```

### MCP Resources

The MKP server provides access to Kubernetes resources through MCP resources. The resource URIs follow these formats:

- Clustered resources: `k8s://clustered/{group}/{version}/{resource}/{name}`
- Namespaced resources: `k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}`

### Configuration

#### Controlling Resource Discovery

By default, MKP serves all Kubernetes resources as MCP resources, which provides useful context for LLMs. However, in large clusters with many resources, this can consume significant context space in the LLM.

You can disable this behavior by using the `--serve-resources` flag:

```bash
# Run without serving cluster resources
./build/mkp-server --serve-resources=false

# Run with a specific kubeconfig without serving cluster resources
./build/mkp-server --kubeconfig=/path/to/kubeconfig --serve-resources=false
```

Even with resource discovery disabled, the MCP tools (`get_resource`, `list_resources`, and `apply_resource`) remain fully functional, allowing you to interact with your Kubernetes cluster.

#### Enabling Write Operations

By default, MKP operates in read-only mode, meaning it does not allow write operations on the cluster, i.e. the `apply_resource` tool will not be available. You can enable write operations by using the `--read-write` flag:

```bash
# Run with write operations enabled
./build/mkp-server --read-write=true

# Run with a specific kubeconfig and write operations enabled
./build/mkp-server --kubeconfig=/path/to/kubeconfig --read-write=true
```

### Rate Limiting

MKP includes a built-in rate limiting mechanism to protect the server from excessive API calls, which is particularly important when used with AI agents. The rate limiter uses a token bucket algorithm and applies different limits based on the operation type:

- Read operations (list_resources, get_resource): 120 requests per minute
- Write operations (apply_resource, delete_resource): 30 requests per minute
- Default for other operations: 60 requests per minute

Rate limits are applied per client session, ensuring fair resource allocation across multiple clients. The rate limiting feature can be enabled or disabled via the command line flag:

```bash
# Run with rate limiting enabled (default)
./build/mkp-server

# Run with rate limiting disabled
./build/mkp-server --enable-rate-limiting=false
```

## Development

### Running tests

```bash
task test
```

### Formatting code

```bash
task fmt
```

### Linting code

```bash
task lint
```

### Updating dependencies

```bash
task deps
```

## Running as an MCP Server with ToolHive

MKP can be run as a Model Context Protocol (MCP) server using [ToolHive](https://github.com/StacklokLabs/toolhive), which simplifies the deployment and management of MCP servers.

### Prerequisites

1. Install ToolHive by following the [installation instructions](https://github.com/StacklokLabs/toolhive#installation).
2. Ensure you have Docker or Podman installed on your system.
3. Configure your Kubernetes credentials (kubeconfig) for the cluster you want to interact with.

### Running MKP with ToolHive

To run MKP as an MCP server using ToolHive:

```bash
# Run the MKP server using the published container image
thv run --name mkp --transport sse --target-port 8080 --volume $HOME/.kube:/home/nonroot/.kube:ro ghcr.io/stackloklabs/mkp/server:latest
```

This command:
- Names the server instance "mkp"
- Uses the SSE transport protocol
- Mounts your local kubeconfig into the container (read-only)
- Uses the latest published MKP image from GitHub Container Registry

To use a specific version instead of the latest:

```bash
thv run --name mkp --transport sse --target-port 8080 --volume $HOME/.kube:/home/nonroot/.kube:ro ghcr.io/stackloklabs/mkp/server:v0.0.1
```

### Verifying the MKP Server is Running

To verify that the MKP server is running:

```bash
thv list
```

This will show all running MCP servers managed by ToolHive, including the MKP server.

### Stopping the MKP Server

To stop the MKP server:

```bash
thv stop mkp
```

To remove the server instance completely:

```bash
thv rm mkp
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.