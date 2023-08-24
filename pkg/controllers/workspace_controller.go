package controllers

import (
	"context"

	"github.com/go-logr/logr"
	kdmv1alpha1 "github.com/kdm/api/v1alpha1"
	"github.com/kdm/pkg/machine"
	"github.com/kdm/pkg/node"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type WorkspaceReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (c *WorkspaceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	workspaceObj := &kdmv1alpha1.Workspace{}
	if err := c.Client.Get(ctx, req.NamespacedName, workspaceObj); err != nil {
		if !errors.IsNotFound(err) {
			klog.ErrorS(err, "failed to get workspace", "workspace", req.Name)
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	klog.InfoS("Reconciling", "workspace", req.NamespacedName)
	if !apimeta.IsStatusConditionFalse(workspaceObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeReady)) {
		err := c.setWorkspaceStatusCondition(ctx, workspaceObj, kdmv1alpha1.WorkspaceConditionTypeReady, metav1.ConditionFalse,
			"WorkspacePending", "starting provisioning workspace")
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Handle deleting workspace, garbage collect all the resources.
	if !workspaceObj.DeletionTimestamp.IsZero() {
		klog.InfoS("the workspace is in the process of being deleted", "workspace", klog.KObj(workspaceObj))
		if !apimeta.IsStatusConditionTrue(workspaceObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeDeleting)) {
			err := c.setWorkspaceStatusCondition(ctx, workspaceObj, kdmv1alpha1.WorkspaceConditionTypeDeleting, metav1.ConditionTrue,
				"WorkspaceDeleted", "workspace is being deleted")
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		return c.garbageCollectWorkspace(ctx, workspaceObj)
	}

	// Read ResourceSpec
	err := c.applyWorkspaceResource(ctx, workspaceObj)
	if err != nil {
		return reconcile.Result{}, err
	}
	// TODO apply InferenceSpec
	// TODO apply TrainingSpec
	if !apimeta.IsStatusConditionTrue(workspaceObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeReady)) {
		err = c.setWorkspaceStatusCondition(ctx, workspaceObj, kdmv1alpha1.WorkspaceConditionTypeReady, metav1.ConditionTrue,
			"WorkspaceReady", "workspace is ready")
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (c *WorkspaceReconciler) applyWorkspaceResource(ctx context.Context, wObj *kdmv1alpha1.Workspace) error {
	klog.InfoS("applyWorkspaceResource", "workspace", klog.KObj(wObj))
	if !apimeta.IsStatusConditionFalse(wObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeResourceProvisioned)) {
		err := c.setWorkspaceStatusCondition(ctx, wObj, kdmv1alpha1.WorkspaceConditionTypeResourceProvisioned, metav1.ConditionFalse,
			"WorkspaceResourceDeploying", "starting provisioning workspace resource")
		if err != nil {
			return err
		}
	}
	validNodeList := []*corev1.Node{}

	// Check the current cluster nodes if they match the labelSelector and instanceType
	validCurrentClusterNodeList, err := c.validateCurrentClusterNodes(ctx, wObj)
	if err != nil {
		return err
	}

	// Check preferredNodes
	preferredList := wObj.Resource.PreferredNodes
	if preferredList == nil {
		klog.InfoS("PreferredNodes list is empty")
	} else {
		for n := range preferredList {
			lo.Find(validNodeList, func(nodeItem *corev1.Node) bool {
				if nodeItem.Name == preferredList[n] {
					validNodeList = append(validNodeList, nodeItem)
					return true
				}
				// else do nothing for now
				return false
			})
		}
	}
	// TODO check nodes in the WorkspaceStatus.WorkerNodes.

	for n := range validCurrentClusterNodeList {
		if len(validNodeList) == lo.FromPtr(wObj.Resource.Count) {
			break
		}
		_, found := lo.Find(validNodeList, func(nodeItem *corev1.Node) bool {
			return nodeItem.Name == validCurrentClusterNodeList[n].Name
		})
		if !found {
			validNodeList = append(validNodeList, validCurrentClusterNodeList[n])
		}
	}

	validNodeCount := len(validNodeList)
	// subtract all valid nodes from the desired count
	remainingNodeCount := lo.FromPtr(wObj.Resource.Count) - validNodeCount

	// if current valid nodes Count == workspace count, then all good and return
	if remainingNodeCount == 0 {
		klog.InfoS("number of existing nodes are equal to the required workspace count", "workspace.Count", lo.FromPtr(wObj.Resource.Count))
	} else {
		klog.InfoS("need to create more nodes", "NodeCount", remainingNodeCount)
		for i := 0; i < remainingNodeCount; i++ {
			newNode, err := c.createAndValidateNode(ctx, wObj)
			if err != nil {
				return err
			}
			validNodeList = append(validNodeList, newNode)
		}
	}

	// Ensure all nodes plugins are running successfully
	if !apimeta.IsStatusConditionFalse(wObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeConfigured)) {
		err = c.setWorkspaceStatusCondition(ctx, wObj, kdmv1alpha1.WorkspaceConditionTypeConfigured, metav1.ConditionFalse,
			"NodesConfiguring", "nodes are being configured")
		if err != nil {
			return err
		}
	}
	for i := range validNodeList {
		err = node.EnsureNodePlugins(ctx, validNodeList[i], c.Client)
		if err != nil {
			return err
		}
	}
	if !apimeta.IsStatusConditionTrue(wObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeConfigured)) {
		err = c.setWorkspaceStatusCondition(ctx, wObj, kdmv1alpha1.WorkspaceConditionTypeConfigured, metav1.ConditionTrue,
			"NodesConfigured", "nodes have been configured")
		if err != nil {
			return err
		}
	}
	// Add the valid nodes names to the WorkspaceStatus.WorkerNodes
	err = c.updateWorkspaceStatusWithNodeList(ctx, wObj, validNodeList)
	if err != nil {
		return err
	}

	if !apimeta.IsStatusConditionTrue(wObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeResourceProvisioned)) {
		err = c.setWorkspaceStatusCondition(ctx, wObj, kdmv1alpha1.WorkspaceConditionTypeResourceProvisioned, metav1.ConditionTrue,
			"WorkspaceResourceDeployed", "workspace resource has been provisioned")
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *WorkspaceReconciler) validateCurrentClusterNodes(ctx context.Context, wObj *kdmv1alpha1.Workspace) ([]*corev1.Node, error) {
	klog.InfoS("validateCurrentClusterNodes", "workspace", klog.KObj(wObj))
	var validCurrentNodeList []*corev1.Node
	opt := &client.ListOptions{}
	if wObj.Resource.LabelSelector != nil && len(wObj.Resource.LabelSelector.MatchLabels) != 0 {
		opt.LabelSelector = labels.SelectorFromSet(wObj.Resource.LabelSelector.MatchLabels)

	} else {
		klog.InfoS("no Label Selector sets for the workspace", "workspace", wObj.Name)
		return nil, nil
	}

	nodeList, err := node.ListNodes(ctx, c.Client, opt)
	if err != nil {
		return nil, err
	}
	if len(nodeList.Items) == 0 {
		klog.InfoS("no current nodes match the workspace resource spec", "workspace", wObj.Name)
		return nil, nil
	}

	var foundInstanceType bool
	for index := range nodeList.Items {
		nodeObj := nodeList.Items[index]
		foundInstanceType = c.validateNodeInstanceType(ctx, wObj, lo.ToPtr(nodeObj))
		if foundInstanceType {
			klog.InfoS("found a current valid node", "name", nodeObj.Name)
			validCurrentNodeList = append(validCurrentNodeList, lo.ToPtr(nodeObj))
		}
	}

	klog.InfoS("found current valid nodes", "count", len(validCurrentNodeList))
	return validCurrentNodeList, nil
}

// check if node has the required instanceType
func (c *WorkspaceReconciler) validateNodeInstanceType(ctx context.Context, wObj *kdmv1alpha1.Workspace, nodeObj *corev1.Node) bool {
	klog.InfoS("validateNodeInstanceType", "workspace", klog.KObj(wObj))

	if instanceTypeLabel, found := nodeObj.Labels[corev1.LabelInstanceTypeStable]; found {
		if instanceTypeLabel != wObj.Resource.InstanceType {
			klog.InfoS("node has instance type which does not match the workspace instance type", "node",
				nodeObj.Name, "InstanceType", wObj.Resource.InstanceType)
			return false
		}
	}
	klog.InfoS("node instance type matches the workspace one", "node",
		nodeObj.Name, "InstanceType", wObj.Resource.InstanceType)
	return true
}

func (c *WorkspaceReconciler) createAndValidateNode(ctx context.Context, wObj *kdmv1alpha1.Workspace) (*corev1.Node, error) {
	newMachine := machine.GenerateMachineManifest(ctx, wObj)
	if !apimeta.IsStatusConditionFalse(wObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeMachineProvisioned)) {
		err := c.setWorkspaceStatusCondition(ctx, wObj, kdmv1alpha1.WorkspaceConditionTypeMachineProvisioned, metav1.ConditionFalse,
			"MachineProvisioning", "machines are being created")
		if err != nil {
			return nil, err
		}
	}
	err := machine.CreateMachine(ctx, newMachine, c.Client)
	if err != nil {
		klog.ErrorS(err, "failed to create machine", "machine", newMachine.Name)
		return nil, err
	}
	klog.InfoS("a new machine has been created", "machine", newMachine.Name)

	// check machine status until it's ready
	err = machine.CheckMachineStatus(ctx, newMachine, c.Client)
	if err != nil {
		return nil, err
	}

	nodeName := newMachine.Status.NodeName
	if nodeName == "" {
		// TODO retry get machine
	}
	nodeObj, err := node.GetNode(ctx, nodeName, c.Client)
	if err != nil {
		return nil, err
	}

	if !apimeta.IsStatusConditionTrue(wObj.Status.Conditions, string(kdmv1alpha1.WorkspaceConditionTypeMachineProvisioned)) {
		err = c.setWorkspaceStatusCondition(ctx, wObj, kdmv1alpha1.WorkspaceConditionTypeMachineProvisioned, metav1.ConditionTrue,
			"MachineProvisioned", "machines have been created")
		if err != nil {
			return nil, err
		}
	}
	return nodeObj, nil
}

func (c *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c.Recorder = mgr.GetEventRecorderFor("Workspace")
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&kdmv1alpha1.Workspace{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(c)
}
