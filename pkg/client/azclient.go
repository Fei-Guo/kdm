package client

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/kdm/pkg/auth"
	"github.com/kdm/pkg/utils"
	"k8s.io/klog/v2"
)

type AZClient struct {
	VirtualMachineScaleSetsClient    *armcompute.VirtualMachineScaleSetsClient
	VirtualMachineScaleSetsExtClient *armcompute.VirtualMachineScaleSetExtensionsClient

	NetworkInterfacesClient *armnetwork.VirtualNetworksClient
	SKUClient               *armcompute.ResourceSKUsClient
}

func CreateAzClient(cfg *auth.Config) (*AZClient, error) {
	azClient, err := NewAZClient(cfg)
	if err != nil {
		return nil, err
	}

	return azClient, nil
}

func NewAZClient(cfg *auth.Config) (*AZClient, error) {
	cred, err := auth.NewCredential(cfg)
	if err != nil {
		return nil, err
	}

	opts := utils.DefaultArmOpts()

	vnClient, err := armnetwork.NewVirtualNetworksClient(cfg.SubscriptionID, cred, opts)
	if err != nil {
		return nil, err
	}
	klog.V(5).Infof("Created virtual network client %v using token credential", vnClient)

	virtualMachinesSSClient, err := armcompute.NewVirtualMachineScaleSetsClient(cfg.SubscriptionID, cred, opts)
	if err != nil {
		return nil, err
	}
	klog.V(5).Infof("Created virtual machines SS client %v, using a token credential", virtualMachinesSSClient)
	if err != nil {
		return nil, err
	}

	virtualMachinesSSExtClient, err := armcompute.NewVirtualMachineScaleSetExtensionsClient(cfg.SubscriptionID, cred, opts)
	if err != nil {
		return nil, err
	}
	klog.V(5).Infof("Created virtual machines SS client %v, using a token credential", virtualMachinesSSClient)
	if err != nil {
		return nil, err
	}

	skuClient, err := armcompute.NewResourceSKUsClient(cfg.SubscriptionID, cred, opts)

	klog.V(5).Infof("Created sku client with authorizer: %v", skuClient)

	return &AZClient{
		VirtualMachineScaleSetsClient:    virtualMachinesSSClient,
		VirtualMachineScaleSetsExtClient: virtualMachinesSSExtClient,
		NetworkInterfacesClient:          vnClient,
		SKUClient:                        skuClient,
	}, nil
}

func GetAzConfig() (*auth.Config, error) {
	cfg, err := auth.BuildAzureConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
