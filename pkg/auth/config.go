package auth

import (
	"fmt"
	"os"
)

type Config struct {
	SubscriptionID         string `json:"subscriptionId" yaml:"subscriptionId"`
	UserAssignedIdentityID string `json:"userAssignedIdentityID" yaml:"userAssignedIdentityID"`
}

func (cfg *Config) PrepareConfig() error {
	err := cfg.PrepareSub()
	if err != nil {
		return err
	}

	err = cfg.prepareMSI()
	if err != nil {
		return err
	}
	return nil
}

func (cfg *Config) PrepareSub() error {
	subscriptionID, _ := os.LookupEnv("SUBSCRIPTION_ID")
	if subscriptionID == "" {
		subscriptionID = "ff05f55d-22b5-44a7-b704-f9a8efd493ed"
	}
	cfg.SubscriptionID = subscriptionID
	return nil
}

func (cfg *Config) prepareMSI() error {
	userAssignedIdentityIDFromEnv := os.Getenv("USER_ASSIGNED_IDENTITY_ID")
	if userAssignedIdentityIDFromEnv != "" {
		cfg.UserAssignedIdentityID = userAssignedIdentityIDFromEnv
	}
	return nil
}

func BuildAzureConfig() (*Config, error) {
	var err error
	cfg := &Config{}
	err = cfg.PrepareConfig()
	if err != nil {
		return nil, err
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (cfg *Config) validate() error {
	if cfg.SubscriptionID == "" {
		return fmt.Errorf("subscription ID not set")
	}
	if cfg.UserAssignedIdentityID == "" {
		return fmt.Errorf("USER_ASSIGNED_IDENTITY_ID not set")
	}
	return nil
}

func GetUserAgentExtension() string {
	return fmt.Sprintf("kdm-aks/v%s", "poc")
}
