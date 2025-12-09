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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// Reconcile Loop
func (r *SimpleAppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. Fetch the SimpleApp instance
	var simpleApp appsv1alpha1.SimpleApp
	if err := r.Get(ctx, req.NamespacedName, &simpleApp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Check/Create Deployment
	deployment, err := r.ensureDeployment(ctx, &simpleApp)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 3. Check/Create Service
	_, err = r.ensureService(ctx, &simpleApp)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 4. Update Status
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
	// DYNAMIC: We get replicas from the YAML
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
						Name: "app",
						// DYNAMIC: Here is the magic! We use the variable instead of a hardcoded string
						Image:           cr.Spec.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{{
							// DYNAMIC: We use the port defined by the user
							ContainerPort: cr.Spec.ContainerPort,
						}},
					}},
				},
			},
		},
	}

	// Set ControllerReference
	if err := ctrl.SetControllerReference(cr, dep, r.Scheme); err != nil {
		return nil, err
	}

	// Check if exists
	var existing appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Name: dep.Name, Namespace: dep.Namespace}, &existing)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		// Create
		if err := r.Create(ctx, dep); err != nil {
			return nil, err
		}
		return dep, nil
	}

	// Update Logic (simplified)
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

// ensureService creates or updates the Service
func (r *SimpleAppReconciler) ensureService(ctx context.Context, cr *appsv1alpha1.SimpleApp) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": cr.Name},
			Ports: []corev1.ServicePort{{
				// DYNAMIC: Mapping user ports to service ports
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

	return &existing, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimpleAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.SimpleApp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}