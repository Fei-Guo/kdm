package auth

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// NewCredential provides a token credential for msi and service principal auth
func NewCredential(cfg *Config) (azcore.TokenCredential, error) {
	if cfg == nil {
		return nil, fmt.Errorf("failed to create credential, nil config provided")
	}
	msiCred, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
		ID: azidentity.ClientID(cfg.UserAssignedIdentityID),
		ClientOptions: azcore.ClientOptions{
			Cloud: cloud.AzurePublic,
		}})
	if err != nil {
		return nil, err
	}
	return msiCred, nil
}
