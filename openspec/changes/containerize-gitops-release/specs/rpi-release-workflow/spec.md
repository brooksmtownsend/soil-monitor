# Spec Delta

## REMOVED Requirements

### Requirement: One-command release
**Reason**: Replaced by CI image publication and GitOps rollout.
**Migration**: Merge to the default branch or tag a version; promote the image tag in the GitOps repository.

### Requirement: Configurable Raspberry Pi target
**Reason**: There is no SSH deployment target.
**Migration**: None.

### Requirement: Release preflight
**Reason**: SSH/sudo preflight no longer applies.
**Migration**: None.

### Requirement: Staged artifact installation
**Reason**: Artifacts are delivered as images.
**Migration**: None.

### Requirement: Host configuration is preserved
**Reason**: SSH-based release workflow is removed in favor of image publication and GitOps rollout.
**Migration**: None.

### Requirement: Service activation and verification
**Reason**: SSH-based release workflow is removed in favor of image publication and GitOps rollout.
**Migration**: None.

### Requirement: Failed-release recovery
**Reason**: SSH-based release workflow is removed in favor of image publication and GitOps rollout.
**Migration**: None.

### Requirement: Effect-free dry run
**Reason**: SSH-based release workflow is removed in favor of image publication and GitOps rollout.
**Migration**: None.

### Requirement: Release documentation
**Reason**: SSH-based release workflow is removed in favor of image publication and GitOps rollout.
**Migration**: None.

### Requirement: Bootstrap-assisted first deployment
**Reason**: SSH-based release workflow is removed in favor of image publication and GitOps rollout.
**Migration**: None.

### Requirement: Bootstrap target consistency
**Reason**: SSH-based release workflow is removed in favor of image publication and GitOps rollout.
**Migration**: None.
