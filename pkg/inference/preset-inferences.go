package inference

import (
	"context"
	"fmt"

	kdmv1alpha1 "github.com/kdm/api/v1alpha1"
	"github.com/kdm/pkg/k8sresources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	PresetSetModelllama2AImage = "aimodelsregistry.azurecr.io/llama-2-7b-chat:latest"
	PresetSetModelllama2BImage = "aimodelsregistry.azurecr.io/llama-2-13b-chat:latest"
	PresetSetModelllama2CImage = "aimodelsregistry.azurecr.io/llama-2-70b-chat:latest"

	ProbePath = "/healthz"
	Port5000  = int32(5000)
)

var (
	containerPorts = []corev1.ContainerPort{{
		ContainerPort: Port5000,
	},
	}

	livenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(5000),
				Path: ProbePath,
			},
		},
		InitialDelaySeconds: 60,
		PeriodSeconds:       10,
	}

	readinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(5000),
				Path: ProbePath,
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
	}

	tolerations = []corev1.Toleration{
		{
			Effect:   corev1.TaintEffectNoSchedule,
			Operator: corev1.TolerationOpEqual,
			Key:      k8sresources.GPUString,
		},
		{
			Effect: corev1.TaintEffectNoSchedule,
			Value:  k8sresources.GPUString,
			Key:    "sku",
		},
	}
)

func CreateLLAMA2APresetModel(ctx context.Context, workspaceName, namespace string, labelSelector *v1.LabelSelector, volume []corev1.Volume, kubeClient client.Client) error {
	klog.InfoS("CreateLLAMA2APresetModel")
	commands := []string{
		"/bin/sh",
		"-c",
		"cd /workspace/llama/llama-2-7b-chat && torchrun web_example_chat_completion.py",
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceName(k8sresources.CapacityNvidiaGPU): resource.MustParse("1"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceName(k8sresources.CapacityNvidiaGPU): resource.MustParse("1"),
		},
	}
	volumeMount := []corev1.VolumeMount{}
	if len(volume) != 0 {
		volumeMount = append(volumeMount, corev1.VolumeMount{
			Name:      volume[0].Name,
			MountPath: "/dev/shm",
		})
	}

	depObj := k8sresources.GenerateDeploymentManifest(ctx, fmt.Sprint(workspaceName, string(kdmv1alpha1.PresetSetModelllama2A)), namespace,
		PresetSetModelllama2AImage, 1, labelSelector, commands, containerPorts, livenessProbe, readinessProbe,
		resourceRequirements, volumeMount, tolerations, volume)
	err := k8sresources.CreateDeployment(ctx, depObj, kubeClient)
	if err != nil {
		return err
	}
	return nil
}

func CreateLLAMA2BPresetModel(ctx context.Context, workspaceName, namespace string, labelSelector *v1.LabelSelector, volume []corev1.Volume, kubeClient client.Client) error {
	klog.InfoS("CreateLLAMA2BPresetModel")

	commands := []string{
		"/bin/sh",
		"-c",
		"cd /workspace/llama/llama-2-13b-chat && torchrun --nproc_per_node=2 web_example_chat_completion.py",
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceName(k8sresources.CapacityNvidiaGPU): resource.MustParse("2"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceName(k8sresources.CapacityNvidiaGPU): resource.MustParse("2"),
		},
	}
	volumeMount := []corev1.VolumeMount{}
	if len(volume) != 0 {
		volumeMount = append(volumeMount, corev1.VolumeMount{
			Name:      volume[0].Name,
			MountPath: "/dev/shm",
		})
	}

	depObj := k8sresources.GenerateDeploymentManifest(ctx, fmt.Sprint(workspaceName, string(kdmv1alpha1.PresetSetModelllama2B)), namespace,
		PresetSetModelllama2BImage, 1, labelSelector, commands, containerPorts, livenessProbe, readinessProbe,
		resourceRequirements, volumeMount, tolerations, volume)
	err := k8sresources.CreateDeployment(ctx, depObj, kubeClient)
	if err != nil {
		return err
	}
	return nil
}

func CreateLLAMA2CPresetModel(ctx context.Context, workspaceName, namespace string, labelSelector *v1.LabelSelector, volume []corev1.Volume, kubeClient client.Client) error {
	klog.InfoS("CreateLLAMA2CPresetModel")
	var commands []string

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceName(k8sresources.CapacityNvidiaGPU): resource.MustParse("4"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceName(k8sresources.CapacityNvidiaGPU): resource.MustParse("4"),
		},
	}

	volumeMount := []corev1.VolumeMount{}
	if len(volume) != 0 {
		volumeMount = append(volumeMount, corev1.VolumeMount{
			Name:      volume[0].Name,
			MountPath: "/dev/shm",
		})
	}

	depObj := k8sresources.GenerateDeploymentManifest(ctx, fmt.Sprint(workspaceName, string(kdmv1alpha1.PresetSetModelllama2C)), namespace,
		PresetSetModelllama2CImage, 1, labelSelector, commands, containerPorts, livenessProbe, readinessProbe,
		resourceRequirements, volumeMount, tolerations, volume)

	err := k8sresources.CreateDeployment(ctx, depObj, kubeClient)
	if err != nil {
		return err
	}
	return nil
}
