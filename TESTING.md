# Testing Guide - SimpleApp Operator (Ingress: NGINX and Traefik)

This guide describes how to validate the SimpleApp Operator with NGINX and Traefik ingress controllers.

## Prerequisites
- Kubernetes cluster v1.19+
- kubectl configured
- Pre-built images available locally or in a registry:
  - controller:latest
  - simpleapp-dashboard:v1
  - Optional test image: nginx:latest

## Quick Cluster Check
```bash
kubectl cluster-info
kubectl get nodes
```

## NGINX Ingress Controller Flow

1) Install NGINX
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
kubectl get pods -n ingress-nginx
kubectl get ingressclass   # expect "nginx"
```

2) Deploy operator + dashboard (NGINX variant)
```bash
kubectl apply -k deploy/kustomize/variants/with-nginx
kubectl get deployment,pod -n simple-app-operator-system
kubectl get deployment,pod -n simple-app-dashboard
kubectl get deployment -n simple-app-operator-system simple-app-operator-controller-manager -o yaml | grep -A2 INGRESS_CLASS_NAME
```

3) Create a test app
```bash
cat > /tmp/test-nginx-app.yaml <<EOF_APP
apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: test-nginx-app
  namespace: default
spec:
  image: nginx:latest
  replicas: 2
  containerPort: 80
  servicePort: 80
EOF_APP

kubectl apply -f /tmp/test-nginx-app.yaml
kubectl get deployment,svc,ingress test-nginx-app
kubectl describe ingress test-nginx-app
```

4) Validate routing
```bash
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8080:80 &
curl -H "Host: test-nginx-app.local" http://localhost:8080
```

5) Cleanup (NGINX flow)
```bash
kubectl delete simpleapp test-nginx-app
kubectl get deployment,svc,ingress -l app=test-nginx-app
```

## Traefik Ingress Controller Flow

1) Install Traefik
```bash
helm repo add traefik https://traefik.github.io/charts
helm repo update
helm install traefik traefik/traefik --namespace traefik --create-namespace
kubectl get pods -n traefik
kubectl get ingressclass   # expect "traefik"
```

2) Deploy operator + dashboard (Traefik variant)
```bash
kubectl delete -k deploy/kustomize/variants/with-nginx || true
kubectl apply -k deploy/kustomize/variants/with-traefik
kubectl get deployment,pod -n simple-app-operator-system
kubectl get deployment -n simple-app-operator-system simple-app-operator-controller-manager -o yaml | grep -A2 INGRESS_CLASS_NAME
```

3) Create a test app
```bash
cat > /tmp/test-traefik-app.yaml <<EOF_APP
apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: test-traefik-app
  namespace: default
spec:
  image: nginx:latest
  replicas: 2
  containerPort: 80
  servicePort: 80
EOF_APP

kubectl apply -f /tmp/test-traefik-app.yaml
kubectl get deployment,svc,ingress test-traefik-app
kubectl describe ingress test-traefik-app
```

4) Validate routing
```bash
kubectl port-forward -n traefik svc/traefik 8080:80 &
curl -H "Host: test-traefik-app.local" http://localhost:8080
```

5) Cleanup (Traefik flow)
```bash
kubectl delete simpleapp test-traefik-app
kubectl get deployment,svc,ingress -l app=test-traefik-app
```

## Parallel Comparison (optional)
```bash
# Install both controllers if resources allow
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
helm install traefik traefik/traefik --namespace traefik --create-namespace

# Deploy operator with one variant at a time
kubectl apply -k deploy/kustomize/variants/with-nginx

# Sample app for NGINX
cat > /tmp/app-nginx.yaml <<EOF_APP
apiVersion: apps.myapp.io/v1
kind: SimpleApp
metadata:
  name: myapp-nginx
  namespace: default
spec:
  image: nginx:latest
  replicas: 1
  containerPort: 80
  servicePort: 80
EOF_APP

kubectl apply -f /tmp/app-nginx.yaml
kubectl port-forward -n ingress-nginx svc/ingress-nginx-controller 8080:80 &
curl -H "Host: myapp-nginx.local" http://localhost:8080
```

## Troubleshooting

Ingress not routing
```bash
kubectl get pods -n ingress-nginx
kubectl get pods -n traefik
kubectl get ingress <name> -o yaml | grep ingressClassName
kubectl logs -n simple-app-operator-system -l control-plane=controller-manager -f
kubectl get deployment -n simple-app-operator-system simple-app-operator-controller-manager -o yaml | grep INGRESS_CLASS_NAME
```

Service unreachable
```bash
kubectl get svc <name>
kubectl get endpoints <name>
kubectl get pods -l app=<name>
kubectl port-forward pod/<pod> 3000:80 &
curl http://localhost:3000
```

Ingress controller not picking up ingress
```bash
# NGINX
kubectl exec -n ingress-nginx deployment/ingress-nginx-controller -- cat /etc/nginx/nginx.conf | grep <name>
# Traefik
kubectl logs -n traefik -l app.kubernetes.io/name=traefik | grep <name>
```

## Final Validation
- Ingress controller routes .local hosts (or DNS you configured)
- Ingress resources are created automatically by the operator
- INGRESS_CLASS_NAME matches the chosen controller
- Dashboard reachable and can create SimpleApp

Dashboard access (direct port-forward)
```bash
kubectl port-forward -n simple-app-dashboard svc/dashboard-service 3000:80 &
# http://localhost:3000
```

## Cleanup
```bash
kubectl delete simpleapp --all
kubectl delete -k deploy/kustomize/variants/with-nginx || true
kubectl delete -k deploy/kustomize/variants/with-traefik || true
helm uninstall traefik -n traefik || true
kubectl delete -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml || true
```
