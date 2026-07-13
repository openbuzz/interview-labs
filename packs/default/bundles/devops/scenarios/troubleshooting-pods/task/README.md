# Payments — Payment Gateway

The `payment-gateway` deployment in namespace `payments` has pods that are not starting on its first rollout.

Investigate the issue, fix it, and confirm all pods are running.

## Verify

```bash
kubectl get pods -n payments
```

All 3 pods should show `Running` with `1/1` ready.
