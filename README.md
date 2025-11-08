# MKP - Model Kontext Protocol Server for Kubernetes

<p align="center">
  <img src="docs/assets/mkp-logo.png" width="400" alt="MKP Logo">
</p>

MKP is a Model Context Protocol (MCP) server for Kubernetes that allows
LLM-powered applications to interact with Kubernetes clusters. It provides tools
for listing and applying Kubernetes resources through the MCP protocol.

## Features

- List resources supported by the Kubernetes API server
- List clustered resources
- List namespaced resources
- Get resources and their subresources (including status, scale, logs, etc.)
- Apply (create or update) clustered resources
- Apply (create or update) namespaced resources
- Execute commands in pods with timeout control
- Generic and pluggable implementation using API Machinery's unstructured client
- Built-in rate limiting for protection against excessive API calls

## Why MKP?

MKP offers several key advantages as a Model Context Protocol server for
Kubernetes:

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

To run the server on a specific port:

```bash
MCP_PORT=9091 task run
```

## Running with ToolHive

MKP can be run as a Model Context Protocol (MCP) server using
[ToolHive](https://github.com/stacklok/toolhive), which simplifies the
deployment and management of MCP servers.

See the
[ToolHive documentation](https://docs.stacklok.com/toolhive/guides-mcp/k8s) for
detailed instructions on how to set up MKP with the ToolHive UI, CLI, or
Kubernetes operator.

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

By default, pod logs are limited to the last 100 lines and 32KB to avoid
overwhelming the LLM's context window. These defaults can be overridden using
the parameters above.

Available parameters for regular resources:

- `resourceVersion`: When specified, shows the resource at that particular
  version

#### list_resources

Lists Kubernetes resources of a specific type.

Parameters:

- `resource_type` (required): Type of resource to list (clustered or namespaced)
- `group`: API group (e.g., apps, networking.k8s.io)
- `version` (required): API version (e.g., v1, v1beta1)
- `resource` (required): Resource name (e.g., deployments, services)
- `namespace`: Namespace (required for namespaced resources)
- `label_selector`: Kubernetes label selector for filtering resources (optional)
- `include_annotations`: Whether to include annotations in the output (default:
  true)
- `exclude_annotation_keys`: List of annotation keys to exclude from output
  (supports wildcards with \*)
- `include_annotation_keys`: List of annotation keys to include in output (if
  specified, only these are included)

##### Annotation Filtering

The `list_resources` tool provides powerful annotation filtering capabilities to
control metadata output size and prevent truncation issues with large
annotations (such as GPU node annotations).

**Basic Usage:**

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

**Exclude specific annotations (useful for GPU nodes):**

```json
{
  "name": "list_resources",
  "arguments": {
    "resource_type": "clustered",
    "group": "",
    "version": "v1",
    "resource": "nodes",
    "exclude_annotation_keys": [
      "nvidia.com/*",
      "kubectl.kubernetes.io/last-applied-configuration"
    ]
  }
}
```

**Include only specific annotations:**

```json
{
  "name": "list_resources",
  "arguments": {
    "resource_type": "namespaced",
    "group": "",
    "version": "v1",
    "resource": "pods",
    "namespace": "default",
    "include_annotation_keys": ["app", "version", "prometheus.io/scrape"]
  }
}
```

**Disable annotations completely for maximum performance:**

```json
{
  "name": "list_resources",
  "arguments": {
    "resource_type": "namespaced",
    "group": "",
    "version": "v1",
    "resource": "pods",
    "namespace": "default",
    "include_annotations": false
  }
}
```

**Annotation Filtering Rules:**

- By default, `kubectl.kubernetes.io/last-applied-configuration` is excluded to
  prevent large configuration data
- `exclude_annotation_keys` supports wildcard patterns using `*` (e.g.,
  `nvidia.com/*` excludes all NVIDIA annotations)
- When `include_annotation_keys` is specified, it takes precedence and only
  those annotations are included
- Setting `include_annotations: false` completely removes all annotations from
  the output
- Wildcard patterns only support `*` at the end of the key (e.g.,
  `nvidia.com/*`)

#### apply_resource

Applies (creates or updates) a Kubernetes resource.

Parameters:

- `resource_type` (required): Type of resource to apply (clustered or
  namespaced)
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

#### post_resource

Posts to a Kubernetes resource or its subresource, particularly useful for
executing commands in pods.

Parameters:

- `resource_type` (required): Type of resource to post to (clustered or
  namespaced)
- `group`: API group (e.g., apps, networking.k8s.io)
- `version` (required): API version (e.g., v1, v1beta1)
- `resource` (required): Resource name (e.g., deployments, services)
- `namespace`: Namespace (required for namespaced resources)
- `name` (required): Name of the resource to post to
- `subresource`: Subresource to post to (e.g., exec)
- `body` (required): Body to post to the resource
- `parameters`: Optional parameters for the request

Example of executing a command in a pod:

```json
{
  "name": "post_resource",
  "arguments": {
    "resource_type": "namespaced",
    "group": "",
    "version": "v1",
    "resource": "pods",
    "namespace": "default",
    "name": "my-pod",
    "subresource": "exec",
    "body": {
      "command": ["ls", "-la", "/"],
      "container": "my-container",
      "timeout": 30
    }
  }
}
```

The `body` for pod exec supports the following fields:

- `command` (required): Command to execute, either as a string or an array of
  strings
- `container` (optional): Container name to execute the command in (defaults to
  the first container)
- `timeout` (optional): Timeout in seconds (defaults to 15 seconds, maximum 60
  seconds)

Note on timeouts:

- Default timeout: 15 seconds if not specified
- Maximum timeout: 60 seconds (any larger value will be capped)
- Commands that exceed the timeout will be terminated and return a timeout error

The response includes stdout, stderr, and any error message:

```json
{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "my-pod",
    "namespace": "default"
  },
  "spec": {
    "command": ["ls", "-la", "/"]
  },
  "status": {
    "stdout": "total 48\ndrwxr-xr-x   1 root root 4096 May  5 14:30 .\ndrwxr-xr-x   1 root root 4096 May  5 14:30 ..\n...",
    "stderr": "",
    "error": ""
  }
}
```

### MCP Resources

The MKP server provides access to Kubernetes resources through MCP resources.
The resource URIs follow these formats:

- Clustered resources: `k8s://clustered/{group}/{version}/{resource}/{name}`
- Namespaced resources:
  `k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}`

### Configuration

#### Transport Protocol

MKP supports two transport protocols for the MCP server:

- **SSE (Server-Sent Events)**: The default transport protocol, suitable for
  most use cases
- **Streamable HTTP**: A streaming HTTP transport that supports both direct HTTP
  responses and SSE streams, useful for environments like ToolHive that require
  HTTP-based communication

You can configure the transport protocol using either a CLI flag or an
environment variable:

```bash
# Using CLI flag
./build/mkp-server --transport=streamable-http

# Using environment variable
MCP_TRANSPORT=streamable-http ./build/mkp-server

# Default (SSE)
./build/mkp-server
```

The `MCP_TRANSPORT` environment variable is automatically set by ToolHive when
running MKP in that environment.

#### Controlling Resource Discovery

By default, MKP serves all Kubernetes resources as MCP resources, which provides
useful context for LLMs. However, in large clusters with many resources, this
can consume significant context space in the LLM.

You can disable this behavior by using the `--serve-resources` flag:

```bash
# Run without serving cluster resources
./build/mkp-server --serve-resources=false

# Run with a specific kubeconfig without serving cluster resources
./build/mkp-server --kubeconfig=/path/to/kubeconfig --serve-resources=false
```

Even with resource discovery disabled, the MCP tools (`get_resource`,
`list_resources`, `apply_resource`, `delete_resource`, and `post_resource`)
remain fully functional, allowing you to interact with your Kubernetes cluster.

#### Enabling Write Operations

By default, MKP operates in read-only mode, meaning it does not allow write
operations on the cluster, i.e. the `apply_resource`, `delete_resource`, and
`post_resource` tools will not be available. You can enable write operations by
using the `--read-write` flag:

```bash
# Run with write operations enabled
./build/mkp-server --read-write=true

# Run with a specific kubeconfig and write operations enabled
./build/mkp-server --kubeconfig=/path/to/kubeconfig --read-write=true
```

### Rate Limiting

MKP includes a built-in rate limiting mechanism to protect the server from
excessive API calls, which is particularly important when used with AI agents.
The rate limiter uses a token bucket algorithm and applies different limits
based on the operation type:

- Read operations (list_resources, get_resource): 120 requests per minute
- Write operations (apply_resource, delete_resource): 30 requests per minute
- Default for other operations: 60 requests per minute

Rate limits are applied per client session, ensuring fair resource allocation
across multiple clients. The rate limiting feature can be enabled or disabled
via the command line flag:

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

## Contributing

We welcome contributions to this MCP server! If you'd like to contribute, please
review the [CONTRIBUTING guide](./CONTRIBUTING.md) for details on how to get
started.

If you run into a bug or have a feature request, please
[open an issue](https://github.com/StacklokLabs/mkp/issues) in the repository or
join us in the `#mcp-servers` channel on our
[community Discord server](https://discord.gg/stacklok).

## License

This project is licensed under the Apache v2 License - see the LICENSE file for
details.
