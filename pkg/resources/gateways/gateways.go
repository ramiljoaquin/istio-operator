/*
Copyright 2019 Banzai Cloud.

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

package gateways

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/goph/emperror"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	istiov1beta1 "github.com/banzaicloud/istio-operator/pkg/apis/istio/v1beta1"
	"github.com/banzaicloud/istio-operator/pkg/k8sutil"
	"github.com/banzaicloud/istio-operator/pkg/resources"
	"github.com/banzaicloud/istio-operator/pkg/util"
)

const (
	componentName      = "gateways"
	ingress            = "ingressgateway"
	egress             = "egressgateway"
	defaultGatewayName = "istio-autogenerated-k8s-ingress"
)

type Reconciler struct {
	resources.Reconciler
	dynamic dynamic.Interface
}

func New(client client.Client, dc dynamic.Interface, config *istiov1beta1.Istio) *Reconciler {
	return &Reconciler{
		Reconciler: resources.Reconciler{
			Client: client,
			Config: config,
		},
		dynamic: dc,
	}
}

func (r *Reconciler) Reconcile(log logr.Logger) error {
	log = log.WithValues("component", componentName)

	log.Info("Reconciling")

	var rsv = []resources.ResourceVariation{
		r.serviceAccount,
		r.clusterRole,
		r.clusterRoleBinding,
		r.deployment,
		r.service,
		r.horizontalPodAutoscaler,
	}
	if r.Config.Spec.DefaultPodDisruptionBudget.Enabled {
		rsv = append(rsv, r.podDisruptionBudget)
	}
	for _, res := range append(resources.ResolveVariations(ingress, rsv), resources.ResolveVariations(egress, rsv)...) {
		o := res()
		err := k8sutil.Reconcile(log, r.Client, o)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile resource", "resource", o.GetObjectKind().GroupVersionKind())
		}
	}

	drs := make([]resources.DynamicResourceWithDesiredState, 0)
	if r.Config.Spec.Gateways.K8sIngress.Enabled {
		drs = append(drs, resources.DynamicResourceWithDesiredState{DynamicResource: r.gateway})
	}
	for _, dr := range drs {
		o := dr.DynamicResource()
		err := o.Reconcile(log, r.dynamic, dr.DesiredState)
		if err != nil {
			return emperror.WrapWith(err, "failed to reconcile dynamic resource", "resource", o.Gvr)
		}
	}

	log.Info("Reconciled")
	return nil
}

func (r *Reconciler) getGatewayConfig(gw string) *istiov1beta1.GatewayConfiguration {
	switch gw {
	case ingress:
		return &r.Config.Spec.Gateways.IngressConfig
	case egress:
		return &r.Config.Spec.Gateways.EgressConfig
	}
	return nil
}

func serviceAccountName(gw string) string {
	return fmt.Sprintf("istio-%s-service-account", gw)
}

func clusterRoleName(gw string) string {
	return fmt.Sprintf("istio-%s-cluster-role", gw)
}

func clusterRoleBindingName(gw string) string {
	return fmt.Sprintf("istio-%s-cluster-role-binding", gw)
}

func gatewayName(gw string) string {
	return fmt.Sprintf("istio-%s", gw)
}

func hpaName(gw string) string {
	return fmt.Sprintf("istio-%s-autoscaler", gw)
}

func pdbName(gw string) string {
	return fmt.Sprintf("istio-%s", gw)
}

func gwLabels(gw string) map[string]string {
	return map[string]string{
		"app": fmt.Sprintf("istio-%s", gw),
	}
}

func labelSelector(gw string) map[string]string {
	return util.MergeLabels(gwLabels(gw), map[string]string{
		"istio": gw,
	})
}
