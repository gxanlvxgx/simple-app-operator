/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/gxanlvxgx/simple-app-operator/api/v1"
)

// SimpleAppReconciler reconciles a SimpleApp object
type SimpleAppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// RBAC Permissions
//+kubebuilder:rbac:groups=apps.myapp.io,resources=simpleapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.myapp.io,resources=simpleapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.myapp.io,resources=simpleapps/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SimpleAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. Fetch the SimpleApp instance
	var simpleApp appsv1alpha1.SimpleApp
	if err := r.Get(ctx, req.NamespacedName, &simpleApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Ensure the Deployment exists and matches the desired state
	deployment, err := r.ensureDeployment(ctx, &simpleApp)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 3. Ensure the Service exists and matches the desired state
	_, err = r.ensureService(ctx, &simpleApp)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 4. Ensure the Ingress exists and matches the desired state (Infrastructure agnostic)
	_, err = r.ensureIngress(ctx, &simpleApp)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 5. Update CR Status with the current state of the Deployment
	if simpleApp.Status.ReadyReplicas != deployment.Status.ReadyReplicas {
		simpleApp.Status.ReadyReplicas = deployment.Status.ReadyReplicas
		if err := r.Status().Update(ctx, &simpleApp); err != nil {
			return ctrl.Result{}, err
		}
	}

	log.Info("Successfully reconciled SimpleApp", "Name", simpleApp.Name, "Image", simpleApp.Spec.Image)
	return ctrl.Result{}, nil
}

// ensureDeployment creates or updates the Deployment based on the CR specs.
func (r *SimpleAppReconciler) ensureDeployment(ctx context.Context, cr *appsv1alpha1.SimpleApp) (*appsv1.Deployment, error) {
	desiredReplicas := cr.Spec.Replicas

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &desiredReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": cr.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": cr.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "app",
						Image:           cr.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{{
							ContainerPort: cr.Spec.ContainerPort,
						}},
					}},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(cr, dep, r.Scheme); err != nil {
		return nil, err
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Name: dep.Name, Namespace: dep.Namespace}, &existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		if err := r.Create(ctx, dep); err != nil {
			return nil, err
		}
		return dep, nil
	}

	needsUpdate := false
	if *existing.Spec.Replicas != *dep.Spec.Replicas {
		needsUpdate = true
	}
	if existing.Spec.Template.Spec.Containers[0].Image != dep.Spec.Template.Spec.Containers[0].Image {
		needsUpdate = true
	}

	if needsUpdate {
		existing.Spec.Replicas = dep.Spec.Replicas
		existing.Spec.Template.Spec.Containers[0].Image = dep.Spec.Template.Spec.Containers[0].Image
		if err := r.Update(ctx, &existing); err != nil {
			return nil, err
		}
	}

	return &existing, nil
}

// ensureService creates or updates the Service to expose the application.
func (r *SimpleAppReconciler) ensureService(ctx context.Context, cr *appsv1alpha1.SimpleApp) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": cr.Name},
			Ports: []corev1.ServicePort{{
				Port:       cr.Spec.ServicePort,
				TargetPort: intstr.FromInt(int(cr.Spec.ContainerPort)),
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	if err := ctrl.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return nil, err
	}

	var existing corev1.Service
	err := r.Get(ctx, client.ObjectKey{Name: svc.Name, Namespace: svc.Namespace}, &existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		if err := r.Create(ctx, svc); err != nil {
			return nil, err
		}
		return svc, nil
	}

	if existing.Spec.Ports[0].Port != svc.Spec.Ports[0].Port ||
		existing.Spec.Ports[0].TargetPort != svc.Spec.Ports[0].TargetPort {
		existing.Spec.Ports = svc.Spec.Ports
		if err := r.Update(ctx, &existing); err != nil {
			return nil, err
		}
	}

	return &existing, nil
}

// ensureIngress manages the Ingress creation based on the environment variable INGRESS_CLASS_NAME.
func (r *SimpleAppReconciler) ensureIngress(ctx context.Context, cr *appsv1alpha1.SimpleApp) (*networkingv1.Ingress, error) {
	// Retrieve the Ingress class name from the environment variable injected by Kustomize
	ingressClassName := os.Getenv("INGRESS_CLASS_NAME")

	// If the environment variable is not set, skip Ingress creation
	if ingressClassName == "" {
		return nil, nil
	}

	pathType := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-ingress",
			Namespace: cr.Namespace,
			Annotations: map[string]string{
				// Legacy annotation for compatibility
				"kubernetes.io/ingress.class": ingressClassName,
			},
		},
		Spec: networkingv1.IngressSpec{
			// Assign the dynamic Ingress class
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					// Generate a local host domain for testing
					Host: cr.Name + ".local",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: cr.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: cr.Spec.ServicePort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set ControllerReference
	if err := ctrl.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return nil, err
	}

	// Check if Ingress already exists
	var existing networkingv1.Ingress
	err := r.Get(ctx, client.ObjectKey{Name: ingress.Name, Namespace: ingress.Namespace}, &existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		// Create Ingress
		if err := r.Create(ctx, ingress); err != nil {
			return nil, err
		}
		return ingress, nil
	}

	// Update Logic: If the Ingress class has changed, update the resource
	if existing.Spec.IngressClassName != nil && *existing.Spec.IngressClassName != ingressClassName {
		existing.Spec.IngressClassName = &ingressClassName
		if err := r.Update(ctx, &existing); err != nil {
			return nil, err
		}
	}

	return &existing, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimpleAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.SimpleApp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
