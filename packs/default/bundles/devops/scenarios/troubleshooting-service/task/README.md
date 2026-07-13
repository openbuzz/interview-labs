# Catalog — Product API

The `product-api-svc` service in namespace `catalog` should route traffic to the `product-api` deployment, but requests are failing. The pods themselves are running fine.

Investigate why the service is not routing traffic and fix the issues.

## Verify

Port-forward the application port locally and test it with `curl`.

You should see the default nginx welcome page.
