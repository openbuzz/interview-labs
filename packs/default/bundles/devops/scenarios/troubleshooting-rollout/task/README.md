# Notifications — Email Service

The `email-service` deployment in namespace `notifications` was being updated to a new image version, but the rollout appears to be stuck. Some pods are running the old version while new pods are not becoming ready.

Investigate the stuck rollout, restore service to a healthy state, then complete the update successfully.

## Verify

```bash
kubectl get pods -n notifications
```

All 3 pods should be running the new image version with `1/1` ready, and the rollout should be complete.
