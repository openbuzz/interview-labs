# Payment Gateway — Solution

## Objectives

1. Identify why pods are not starting.
2. Find the incorrect ConfigMap reference.
3. Fix the deployment to use the correct ConfigMap name.
4. Verify all pods recover.

## Root cause

The deployment `payment-gateway` mounts a ConfigMap volume referencing `gateway-cfg`, but the actual ConfigMap is named `gateway-config`. Since the referenced ConfigMap does not exist, new pods get stuck in `ContainerCreating` with a `FailedMount` warning. The rolling update cannot progress — old pods remain running while new pods never start.

## Diagnosis

Check pod status and events:

```bash
kubectl get pods -n payments
kubectl describe pod <pod-name> -n payments
```

The events show a warning: `MountVolume.SetUp failed for volume "config" : configmap "gateway-cfg" not found`.

List available ConfigMaps to find the correct name:

```bash
kubectl get configmap -n payments
```

This shows `gateway-config` exists — the deployment references `gateway-cfg` instead.

## Fix

Edit the deployment to correct the ConfigMap name:

```bash
kubectl edit deployment payment-gateway -n payments
```

Change `configMap.name` from `gateway-cfg` to `gateway-config` in the volumes section.

Alternatively, patch the deployment:

```bash
kubectl patch deployment payment-gateway -n payments --type=json \
  -p='[{"op":"replace","path":"/spec/template/spec/volumes/0/configMap/name","value":"gateway-config"}]'
```

## Expected result

- New pods start successfully and old pods terminate.
- `kubectl get pods -n payments` shows 3/3 pods ready.
