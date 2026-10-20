# kubernetes-gitops-deployment Specification

## Purpose
Define the Kubernetes manifests and runtime behavior shipped with the project so the application can be rolled out and updated by a GitOps controller.

## Requirements

### Requirement: Declarative Kustomize base
The project SHALL provide a Kustomize base under `deploy/kubernetes/` defining a Deployment, a ClusterIP Service for the dashboard, and a PersistentVolumeClaim for application data, with no environment-specific values (registry credentials, hostnames, broker address) hard-coded beyond documented defaults.

#### Scenario: Base renders
- **WHEN** `kustomize build deploy/kubernetes` is run
- **THEN** valid manifests render without requiring a live cluster

### Requirement: Single writer with persistent history
The Deployment SHALL run exactly one replica using a non-overlapping rollout strategy, and SHALL mount the PersistentVolumeClaim at the database directory so history, plant metadata, and photos survive pod restarts and image upgrades.

#### Scenario: Image is upgraded
- **WHEN** the Deployment image tag changes
- **THEN** the old pod stops before the new pod starts and the new pod opens the existing database unchanged

### Requirement: Hardened pod
The pod SHALL run non-root with a read-only root filesystem, no privilege escalation, all capabilities dropped, and the runtime-default seccomp profile.

#### Scenario: Pod spec is reviewed
- **WHEN** the rendered Deployment is inspected
- **THEN** it declares those security settings and no host devices or privileged mode

### Requirement: Configuration through environment
Runtime configuration SHALL be supplied via environment variables (including a documented, overridable `SOIL_MQTT_BROKER`), and the base SHALL NOT contain secrets.

#### Scenario: Broker is configured per environment
- **WHEN** a GitOps overlay patches `SOIL_MQTT_BROKER`
- **THEN** the application connects to that broker without an image change

### Requirement: Health probes
The Deployment SHALL declare readiness and liveness probes against the dashboard HTTP listener.

#### Scenario: Dashboard unavailable
- **WHEN** the HTTP listener stops responding
- **THEN** the pod is marked not ready and eventually restarted

### Requirement: Image reference by immutable tag
The base SHALL reference the image by repository and a replaceable tag or digest so a GitOps image updater or manual commit can promote versions.

#### Scenario: Promote a version
- **WHEN** the image tag in the GitOps repository is changed to a published tag
- **THEN** the controller rolls the Deployment to that image
