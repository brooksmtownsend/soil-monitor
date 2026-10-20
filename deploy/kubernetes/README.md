# Kubernetes deployment

A Kustomize base for the soil monitor: one Deployment (single replica, `Recreate`), a ClusterIP Service on port 80, and a 1Gi PersistentVolumeClaim holding the SQLite database and plant photos. Images are published to `ghcr.io/brooksmtownsend/wireless-soil-monitor` by `.github/workflows/image.yml` (`sha-<commit>` on `main`, `X.Y.Z` for `vX.Y.Z` tags).

Render locally: `kubectl kustomize deploy/kubernetes`

## Use from a GitOps repository

Reference this directory as a base and patch per environment:

```yaml
# kustomization.yaml in the GitOps repo
resources:
  - github.com/brooksmtownsend/wireless-soil-monitor//deploy/kubernetes?ref=v0.1.0
namespace: soil-monitor
images:
  - name: ghcr.io/brooksmtownsend/wireless-soil-monitor
    newTag: sha-0123abc   # promote by changing this tag
patches:
  - target: {kind: Deployment, name: soil-monitor}
    patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/env/0/value
        value: tcp://mosquitto.messaging.svc:1883
```

Also set `storageClassName` on the PVC if the cluster has no default. Use block storage; SQLite must not live on NFS.

## Notes

- **Broker**: the base only sets `SOIL_MQTT_BROKER`; it does not deploy a broker. The broker listener and the dashboard are unauthenticated and unencrypted, so keep them on a trusted network and put authentication and TLS in front of any Ingress or HTTPRoute.
- **Image pulls**: GHCR packages are private by default. Either make the package public or add an `imagePullSecrets` entry to the Deployment in your overlay.
- **One replica only**: do not scale the Deployment above 1 or switch to `RollingUpdate`.

## Import an existing Pi database

Scale the Deployment to 0, then copy the file into the volume with a throwaway pod (the app image has no shell):

```sh
kubectl scale deploy/soil-monitor --replicas=0
kubectl run db-import --image=busybox --restart=Never --overrides='{"spec":{"securityContext":{"runAsUser":65532,"fsGroup":65532},"containers":[{"name":"db-import","image":"busybox","command":["sleep","3600"],"volumeMounts":[{"name":"data","mountPath":"/data"}]}],"volumes":[{"name":"data","persistentVolumeClaim":{"claimName":"soil-monitor-data"}}]}}'
kubectl wait --for=condition=Ready pod/db-import
kubectl cp soil-monitor.db db-import:/data/soil-monitor.db
kubectl delete pod db-import
kubectl scale deploy/soil-monitor --replicas=1
```

Copy a consistent file from the Pi first, e.g. `sqlite3 /var/lib/soil-monitor/soil-monitor.db ".backup soil-monitor.db"`.
