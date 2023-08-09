package utils

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/kdm/pkg/auth"
)

func DefaultArmOpts() *arm.ClientOptions {
	opts := &arm.ClientOptions{}
	opts.Cloud = cloud.AzurePublic
	opts.Telemetry = DefaultTelemetryOpts()
	opts.Retry = DefaultRetryOpts()
	opts.Transport = defaultHTTPClient
	return opts
}

func DefaultRetryOpts() policy.RetryOptions {
	return policy.RetryOptions{
		MaxRetries: 20,
		// Note the default retry behavior is exponential backoff
		RetryDelay: time.Second * 5,
	}
}

func DefaultTelemetryOpts() policy.TelemetryOptions {
	return policy.TelemetryOptions{
		ApplicationID: auth.GetUserAgentExtension(),
	}
}
