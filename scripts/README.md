# Testing Scripts Guide

Two scripts are provided to validate the SimpleApp Operator:

1. **e2e-test.sh** - Full end-to-end test (recommended)
2. **test-ingress.sh** - Focused ingress validation

---

## E2E Test Script (Recommended)

### What It Does

Automates the entire workflow:

```
1. Check prerequisites (kubectl, curl, helm)
2. Wait for cluster to be accessible
3. Install Ingress Controller (NGINX or Traefik)
4. Deploy Operator + Dashboard
5. Create dashboard SimpleApp
6. Create test SimpleApp
7. Validate dashboard access via Ingress
8. Validate test app access via Ingress
9. Display all created resources
```

### Usage

```bash
# Test with NGINX
./scripts/e2e-test.sh nginx

# Test with Traefik
./scripts/e2e-test.sh traefik
```

### Expected Output

```
===============================================================
[STEP] Checking prerequisites...
[INFO] ✓ kubectl installed
[INFO] ✓ curl installed
...
[STEP] Installing NGINX Ingress Controller...
...
[STEP] Deploying SimpleApp Operator with nginx...
...
[STEP] Creating dashboard SimpleApp...
[INFO] ✓ Dashboard deployment ready
...
[STEP] Testing dashboard access via Ingress...
[INFO] ✓ Dashboard accessible via Ingress!
...
[INFO] ✓✓✓ ALL TESTS COMPLETED SUCCESSFULLY! ✓✓✓
```

### What It Validates

✅ Cluster Connectivity - Kubernetes is accessible
✅ Ingress Controller - NGINX or Traefik installed
✅ Operator Deployment - Controller running with INGRESS_CLASS_NAME set
✅ SimpleApp Creation - CRD works
✅ Ingress Generation - Controller creates Ingress automatically
✅ Ingress Routing - Traffic routing through ingress controller
✅ Dashboard Access - Dashboard reachable via Ingress
✅ Test App Access - Deployed app reachable via Ingress

---

## Ingress Test Script

### What It Does

Single ingress controller test:

```
1. Verify Ingress Controller is installed
2. Verify Operator is deployed
3. Create test SimpleApp
4. Verify Ingress was created automatically
5. Test connectivity
6. Cleanup
```

### Usage

```bash
# Test NGINX
./scripts/test-ingress.sh nginx

# Test Traefik
./scripts/test-ingress.sh traefik

# Test both
./scripts/test-ingress.sh both
```

### When to Use

Use when you only want to test ingress routing, not the full flow. Assumes ingress controller and operator are already installed.

---

## Dashboard Access After Tests

Once the e2e test completes, access the dashboard:

```bash
# Terminal 1 - Port forward
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8080:80
# Or for traefik:
kubectl port-forward -n traefik svc/traefik 8080:80

# Terminal 2 - Browser
# http://platform-dashboard.local:8080
```

### Dashboard Features

Once in the dashboard, you can:

1. **Deployment Form:**
   - Application Name (e.g., `my-webapp`)
   - Docker Image (e.g., `nginx:latest`)
   - Replicas
   - Container Port
   - Service Port
   - Kubernetes Namespace

2. **Submit:** Click "Deploy to Cluster" to execute:
   ```yaml
   apiVersion: apps.myapp.io/v1
   kind: SimpleApp
   metadata:
     name: my-webapp
     namespace: default
   spec:
     image: nginx:latest
     replicas: 1
     containerPort: 80
     servicePort: 80
   ```

3. **Result:** Immediate kubectl output displayed
   - "created" = success
   - "configured" = already exists
   - Errors shown if applicable

### Full Flow

```
Dashboard Form Input
        ↓
Generates SimpleApp YAML
        ↓
kubectl apply via backend
        ↓
Operator Reconciler captures event
        ↓
Creates: Deployment + Service + Ingress
        ↓
Ingress Controller routes traffic
        ↓
Application accessible via browser
```

---

## Troubleshooting

### E2E Script Fails

Verify:
```bash
kubectl cluster-info
kubectl get ingressclass
kubectl get pods -n simple-app-operator-system
kubectl logs -n simple-app-operator-system -l control-plane=controller-manager -f
```

### Dashboard Not Reachable

```bash
kubectl get deployment platform-dashboard
kubectl get ingress platform-dashboard-ingress
kubectl get svc platform-dashboard
kubectl logs -l app=platform-dashboard
```

### SimpleApp Not Creating Ingress

```bash
kubectl logs -n simple-app-operator-system -l control-plane=controller-manager -f | grep INGRESS_CLASS_NAME
kubectl get deployment -n simple-app-operator-system simple-app-operator-controller-manager -o yaml | grep INGRESS_CLASS_NAME
```

---

## Manual Testing Steps

If you prefer manual testing:

### 1. Setup Ingress Controller
```bash
# NGINX
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml

# Traefik
helm install traefik traefik/traefik --namespace traefik --create-namespace
```

### 2. Deploy Operator
```bash
# NGINX variant
kubectl apply -k deploy/kustomize/variants/with-nginx

# Traefik variant
kubectl apply -k deploy/kustomize/variants/with-traefik
```

### 3. Create Test SimpleApp
```bash
cat > test-app.yaml <<EOF
apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: test-app
  namespace: default
spec:
  image: nginx:latest
  replicas: 1
  containerPort: 80
  servicePort: 80
