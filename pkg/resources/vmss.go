package resources

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/kdm/pkg/client"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

func CreateVMSS(ctx context.Context, azClient *client.AZClient) error {

	obj := armcompute.VirtualMachineScaleSet{
		Location: ptr.String("westeurope"),
		Name:     ptr.String("kdm-ss"),
		SKU: &armcompute.SKU{
			Name: ptr.String("standard_nc24ads_a100_v4"),
			Tier: ptr.String("Standard"),
			//CLI:--node-count, RP: profile.Count
			Capacity: ptr.Int64(1),
		},
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			Overprovision: ptr.Bool(false),
			UpgradePolicy: &armcompute.UpgradePolicy{
				Mode: (*armcompute.UpgradeMode)(ptr.String("Manual")),
				AutomaticOSUpgradePolicy: &armcompute.AutomaticOSUpgradePolicy{
					EnableAutomaticOSUpgrade: ptr.Bool(false),
				},
			},
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				OSProfile: &armcompute.VirtualMachineScaleSetOSProfile{
					AdminUsername:      ptr.String("hebaasadmin"),
					AdminPassword:      ptr.String("adminAsH@ba3"),
					ComputerNamePrefix: ptr.String("kdm"),
				},
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &armcompute.ImageReference{
						Publisher: ptr.String("microsoft-dsvm"),
						SKU:       ptr.String("2004-preview-ndv5"),
						Offer:     ptr.String("ubuntu-hpc"),
						Version:   ptr.String("20.04.2023080201"),
					},
				},
				NetworkProfile: &armcompute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: []*armcompute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: ptr.String("kdm-ss"),
							Properties: &armcompute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:            ptr.Bool(true),
								EnableIPForwarding: ptr.Bool(true),
								IPConfigurations: []*armcompute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: ptr.String("kdm-ss"),
										Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
											Subnet: &armcompute.APIEntityReference{
												ID: ptr.String("/subscriptions/ff05f55d-22b5-44a7-b704-f9a8efd493ed/resourceGroups/kdm-poc/providers/Microsoft.Network/virtualNetworks/kdm-vnet/subnets/default"),
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

	if azClient.VirtualMachineScaleSetsClient == nil {
		logging.FromContext(ctx).Error("vmss client is nil")
	}
	response, err := azClient.VirtualMachineScaleSetsClient.BeginCreateOrUpdate(ctx, "kdm-poc",
		"kdm-ss", obj, nil)
	if err != nil {
		logging.FromContext(ctx).Errorf("creating vmss , %s", err)
	}
	if response != nil && response.Done() {
		_, err = response.Result(ctx)
		logging.FromContext(ctx).Errorf("getting vmss response , %s", err)
	}
	return nil
}
