# Email Service — Solution

## Objectives

1. Identify why the rollout is stuck.
2. Rollback to the previous working version.
3. Fix the broken configuration.
4. Complete the update successfully.

## Root cause

The deployment `email-service` was updated with two changes simultaneously:

1. Image changed from `nginx:1.27-alpine` to `nginx:1.28-alpine`.
2. The volumeMount path was changed from `/usr/share/nginx/html` to `/usr/share/nginx/static`.

The ConfigMap content is mounted at the wrong path, so nginx serves its default 404 page instead of the custom HTML. The readiness probe (`GET /` on port 80) fails because nginx returns 404, preventing new pods from becoming ready. With `maxUnavailable: 0`, the old pods remain running while new pods stay stuck.

## Diagnosis

Check the rollout status:

```bash
kubectl rollout status deployment/email-service -n notifications
```

The rollout is stuck — waiting for new pods to become ready.

Check pod status:

```bash
kubectl get pods -n notifications
```

Shows a mix of old running pods and new pods in `0/1 Ready` state.

Describe the deployment to see the current spec:

```bash
kubectl describe deployment email-service -n notifications
```

The volumeMount path is `/usr/share/nginx/static` — nginx expects content at `/usr/share/nginx/html`.

## Fix

Step 1 — Rollback to restore service:

```bash
kubectl rollout undo deployment/email-service -n notifications
kubectl rollout status deployment/email-service -n notifications
```

Wait for the rollback to complete. All pods should be running nginx:1.27-alpine with the correct mount path.

Step 2 — Apply the update with the correct volumeMount path:

```bash
kubectl set image deployment/email-service nginx=nginx:1.28-alpine -n notifications
```

This updates only the image while keeping the correct volumeMount path (`/usr/share/nginx/html`).

Step 3 — Verify the rollout:

```bash
kubectl rollout status deployment/email-service -n notifications
kubectl get pods -n notifications
```

## Expected result

- All 3 pods running nginx:1.28-alpine with `1/1` ready.
- Rollout status shows complete.
- `kubectl rollout history deployment/email-service -n notifications` shows the rollback and successful update.
