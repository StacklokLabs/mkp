package k8s

// Repeated string literals used when building unstructured Kubernetes
// objects or matching well-known resource/subresource names. Extracted
// to satisfy goconst and to make changes (e.g. a typo in a JSON key)
// fail in one place rather than many.
const (
	fieldAPIVersion = "apiVersion"
	fieldKind       = "kind"
	fieldMetadata   = "metadata"
	fieldName       = "name"
	fieldNamespace  = "namespace"
	fieldSpec       = "spec"
	fieldStatus     = "status"
	fieldCommand    = "command"
	fieldStdout     = "stdout"
	fieldStderr     = "stderr"
	fieldError      = "error"
	fieldLogs       = "logs"

	apiVersionV1    = "v1"
	kindPod         = "Pod"
	resourcePods    = "pods"
	subresourceExec = "exec"
)
