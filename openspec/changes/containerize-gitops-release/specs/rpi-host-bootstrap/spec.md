# Spec Delta

## REMOVED Requirements

### Requirement: Fresh-host bootstrap entry point
**Reason**: Host provisioning is replaced by container images deployed through Kubernetes.
**Migration**: Provision nodes outside this project; deploy via `deploy/kubernetes/`.

### Requirement: Idempotent privileged bootstrap
**Reason**: No privileged host mutation remains.
**Migration**: None.

### Requirement: Local MQTT broker provisioning
**Reason**: The broker is an external or in-cluster service.
**Migration**: Run a broker in the cluster and set `SOIL_MQTT_BROKER`.

### Requirement: Hardware and service-account provisioning
**Reason**: The container runs headless as a fixed non-root user.
**Migration**: None.

### Requirement: Initial environment creation and preservation
**Reason**: Configuration moves to Kubernetes environment variables.
**Migration**: Translate `/etc/soil-monitor/soil-monitor.env` values into the GitOps overlay.

### Requirement: Bootstrap diagnostics and security boundaries
**Reason**: Bootstrap is removed.
**Migration**: None.
