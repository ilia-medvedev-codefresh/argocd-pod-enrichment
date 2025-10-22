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
		metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
		"k8s.io/apimachinery/pkg/runtime/schema"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	webhookconsts "argocd-pod-enrichment/pkg/consts/webhook"
	"argocd-pod-enrichment/pkg/kubernetesclient"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	KubernetesClient *kubernetesclient.KubernetesClient
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		log.Error(err, "unable to fetch Pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}


	// Skip reconciliation if the pod is being deleted
	if pod.DeletionTimestamp != nil {
		log.Info("Skipping pod being deleted", "name", pod.Name, "namespace", pod.Namespace)
		return ctrl.Result{}, nil
	}

	// Add or update the annotation
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}

	pod.Annotations["test.codefresh.io/controller"] = "argocd-enrichment"

	argocdApplicationName := pod.Labels[webhookconsts.ApplicationLabelKey]

	if argocdApplicationName == "" {
		log.Info("Pod does not have ArgoCD application label, skipping", "name", pod.Name, "namespace", pod.Namespace)
		return ctrl.Result{}, nil
	}

	argocdApplicationNamespace := pod.Labels[webhookconsts.ApplicationNamespaceLabelKey]

	if argocdApplicationNamespace == "" {
		// Try to get from env
		argocdApplicationNamespace = os.Getenv("ARGOCD_NAMESPACE")
		if argocdApplicationNamespace == "" {
			log.Info("Unable to find ArgoCD app namespace in label or ARGOCD_NAMESPACE env, skipping", "name", pod.Name, "namespace", pod.Namespace)
			return ctrl.Result{}, nil
		} else {
			log.Info("Using ArgoCD app namespace from ARGOCD_NAMESPACE env", "argoNamespace", argocdApplicationNamespace, "name", pod.Name, "namespace", pod.Namespace)
		}
	}

	// Fetch the ArgoCD Application using the dynamic client
	// GVR for ArgoCD Application
	gvr := schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	appObj, err := r.KubernetesClient.DynamicClient.
		Resource(gvr).
		Namespace(argocdApplicationNamespace).
		Get(ctx, argocdApplicationName, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "unable to get ArgoCD Application", "appName", argocdApplicationName, "appNamespace", argocdApplicationNamespace)
		return ctrl.Result{}, nil
	}

	log.Info("Fetched ArgoCD Application", "app", appObj.GetName())

	if appObj.GetAnnotations()["codefresh.io/product"] != "" {
		pod.Labels["codefresh.io/product"] = appObj.GetAnnotations()["codefresh.io/product"]
	}

	if err := r.Update(ctx, &pod); err != nil {
		log.Error(err, "unable to update Pod with annotation")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			labels := obj.GetLabels()
			_, ok := labels[webhookconsts.ApplicationLabelKey]
			return ok
		})).
		Named("pod").
		Complete(r)
}
