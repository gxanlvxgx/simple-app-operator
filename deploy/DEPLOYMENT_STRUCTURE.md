# Deployment Structure

This document describes how the SimpleApp Operator and dashboard are packaged with Kustomize and how they interact with existing ingress controllers.

## Ownership and Scope
- Operator responsibilities: reconcile SimpleApp resources, create Deployment/Service/Ingress, set ingressClassName from INGRESS_CLASS_NAME.
- Operator does not install or manage ingress controllers (NGINX or Traefik must already be present in the cluster).

## Directory Layout
```
deploy/kustomize/
├── bases/
│   ├── controller/   # CRD, RBAC, controller Deployment
│   └── dashboard/    # Dashboard Deployment, Service, RBAC
└── variants/
    ├── with-nginx/   # Overlay: controller + dashboard, INGRESS_CLASS_NAME=nginx
    ├── with-traefik/ # Overlay: controller + dashboard, INGRESS_CLASS_NAME=traefik
    └── production/   # Overlay for tagged releases, higher resources, pull Always
```

## Prerequisites
Install an ingress controller before deploying the operator.
```bash
# NGINX
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml

# Traefik
helm repo add traefik https://traefik.github.io/charts
helm install traefik traefik/traefik --namespace traefik --create-namespace
```

## Overlays
- Development (NGINX): kubectl apply -k deploy/kustomize/variants/with-nginx
- Development (Traefik): kubectl apply -k deploy/kustomize/variants/with-traefik
- Production: kubectl apply -k deploy/kustomize/variants/production

Production overlay specifics:
- ImagePullPolicy Always
- Controller resources: requests 100m/256Mi, limits 1000m/512Mi
- Dashboard resources: requests 100m/256Mi, limits 500m/512Mi
- Image tag set to v1.0.0; resources suffixed with -prod

## RBAC Profile
- Controller: CRUD on SimpleApp, Deployments, Services, Ingress; read Events/ConfigMaps/Secrets; leader election leases.
- Dashboard: CRUD on SimpleApp; read Namespaces for selection.

## Ingress Integration Flow
1. Cluster admin installs NGINX or Traefik.
2. Operator is deployed with the matching overlay; Kustomize sets INGRESS_CLASS_NAME env.
3. For each SimpleApp, the controller creates Deployment, Service, and an Ingress referencing that ingress class.
4. The existing ingress controller picks up the Ingress and configures routing.

## Validation Snippets
```bash
# Render overlays
kustomize build deploy/kustomize/variants/with-nginx
kustomize build deploy/kustomize/variants/with-traefik
kustomize build deploy/kustomize/variants/production

# Dry-run apply
kubectl apply -k deploy/kustomize/variants/with-nginx --dry-run=client -o yaml | less
```

## Operational Notes
- Operator and dashboard are isolated deployments; scale independently.
- Keep ingress controllers in their own namespace; they remain out of operator scope.
- Use tagged images for production; avoid latest.
- Samples are optional; the dashboard can be used to create SimpleApp resources.
