# Kubernetes Deployment

Deploy MKP as a standalone server in a Kubernetes cluster.

## Quick Start

```bash
kubectl apply -f mkp.yaml
```

This creates:
- `mkp` namespace
- ServiceAccount with ClusterRole for read-only access
- Deployment running the MKP server
- ClusterIP Service on port 8080

## Configuration

### Enable Write Operations

1. Uncomment the write verbs in the ClusterRole:
```yaml
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
```

2. Add the `--read-write=true` flag to the Deployment args.

### Serve Cluster Resources

Add `--serve-resources=true` to expose cluster resources as MCP resources.

### Accessing the Server

From within the cluster:
```
http://mkp.mkp.svc.cluster.local:8080
```

For external access, consider using an Ingress or port-forward:
```bash
kubectl port-forward -n mkp svc/mkp 8080:8080
```

## RBAC Customization

The default ClusterRole grants read access to all resources. Modify the `rules` section to restrict access to specific API groups or resources as needed.
