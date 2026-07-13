# Product API — Solution

## Objectives

1. Identify why the service has no endpoints.
2. Fix the selector mismatch between the service and the deployment.
3. Fix the incorrect targetPort.
4. Verify traffic routes correctly.

## Root cause

The service `product-api-svc` has two issues:

1. **Selector mismatch**: The service selects pods with label `app: catalog-api`, but the deployment pods have label `app: product-api`.
2. **Wrong targetPort**: The service targets port `8080`, but nginx listens on port `80`.

## Diagnosis

Check pods are running and note their labels:

```bash
kubectl get pods -n catalog --show-labels
```

Pods are running with label `app=product-api`.

Check the service and its endpoints:

```bash
kubectl describe svc product-api-svc -n catalog
kubectl get endpoints product-api-svc -n catalog
```

The service shows selector `app=catalog-api` and the endpoints list is empty — no pods match the selector. The targetPort is `8080`.

## Fix

Edit the service to correct both issues:

```bash
kubectl edit svc product-api-svc -n catalog
```

1. Change `selector.app` from `catalog-api` to `product-api`.
2. Change `targetPort` from `8080` to `80`.

Alternatively, delete and recreate the service with the correct spec (see `manifests/service.yaml`).

## Expected result

- `kubectl get endpoints product-api-svc -n catalog` shows 3 pod IPs.
- Port-forward and curl return the nginx welcome page:

```bash
kubectl port-forward svc/product-api-svc 8080:80 -n catalog &
curl http://localhost:8080
```
