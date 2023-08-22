package machine

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/kdm/api/v1alpha1"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"knative.dev/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ProvisionerName           = "default"
	LabelGPUProvisionerCustom = "gpu-provisioner.sh/machine-type"
	LabelProvisionerName      = "karpenter.sh/provisioner-name"
	GPUString                 = "gpu"
)

func GenerateMachineManifest(ctx context.Context, workspaceObj *v1alpha1.Workspace) *v1alpha5.Machine {
	klog.InfoS("GenerateMachineManifest", "workspace", klog.KObj(workspaceObj))

	machineName := fmt.Sprint("machine", rand.Intn(5))
	machineLabels := map[string]string{
		LabelProvisionerName:      ProvisionerName,
		LabelGPUProvisionerCustom: GPUString,
	}
	if workspaceObj.Resource.LabelSelector != nil &&
		len(workspaceObj.Resource.LabelSelector.MatchLabels) != 0 {
		machineLabels = lo.Assign(machineLabels, workspaceObj.Resource.LabelSelector.MatchLabels)
	}

	return &v1alpha5.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: workspaceObj.Namespace,
			Labels:    machineLabels,
		},
		Spec: v1alpha5.MachineSpec{
			MachineTemplateRef: &v1alpha5.MachineTemplateRef{
				Name: machineName,
			},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{workspaceObj.Resource.InstanceType},
				},
				{
					Key:      LabelProvisionerName,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{ProvisionerName},
				},
				{
					Key:      LabelGPUProvisionerCustom,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{GPUString},
				},
				{
					Key:      v1.LabelArchStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"amd64"},
				},
				{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"linux"},
				},
			},
			Resources: v1alpha5.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceRequestsCPU: resource.Quantity{
						Format: "2310m",
					},
					v1.ResourceRequestsMemory: resource.Quantity{
						Format: "725280Ki",
					},
				},
			},
			Taints: []v1.Taint{
				{
					Key:    "sku",
					Value:  GPUString,
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
	}
}

func CreateMachine(ctx context.Context, machineObj *v1alpha5.Machine, kubeClient client.Client) error {
	klog.InfoS("CreateMachine", "machine", klog.KObj(machineObj))
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		if apierrors.IsAlreadyExists(err) {
			machineObj.Name = fmt.Sprint("machinetry", rand.Intn(5))
			klog.InfoS("CreateMachine", "machine", klog.KObj(machineObj))
		}
		return true
	}, func() error {
		return kubeClient.Create(ctx, machineObj, &client.CreateOptions{})
	})
}

func CheckMachineStatus(ctx context.Context, machineName, machineNamespace string, kubeClient client.Client) error {
	klog.InfoS("CheckMachineStatus", "machineName", machineName)
	machineObj := &v1alpha5.Machine{}

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, func() error {
		return kubeClient.Get(ctx, client.ObjectKey{Name: machineName, Namespace: machineNamespace}, machineObj, &client.GetOptions{})
	})
	if err != nil {
		klog.ErrorS(err, "failed to get updated machine", "machine", machineName)
		return err
	}

	// check if machine object has conditionType "MachineInitialized" with status "True".
	var found bool
	retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, func() error {
		if _, found = lo.Find(machineObj.GetConditions(), func(condition apis.Condition) bool {
			return condition.Type == v1alpha5.MachineInitialized &&
				condition.Status == v1.ConditionTrue
		}); found {
			return nil
		}
		return fmt.Errorf("machine `%s` condition %s has status false", machineName, v1alpha5.MachineInitialized)
	})

	return nil
}
