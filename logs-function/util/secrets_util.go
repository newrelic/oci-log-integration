package util

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	ociCommon "github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/secrets"

	"github.com/newrelic/oci-log-integration/logs-function/common"
	"github.com/newrelic/oci-log-integration/logs-function/logger"
)

var log = logger.NewLogrusLogger(logger.WithDebugLevel())

// OCISecretsManagerAPI is an interface for interacting with OCI Secrets Manager.
type OCISecretsManagerAPI interface {
	GetSecretBundle(ctx context.Context, request secrets.GetSecretBundleRequest) (secrets.GetSecretBundleResponse, error)
	SetRegion(regionId string)
}

// GetSecretFromOCIVault retrieves a secret from OCI Vault.
// It returns the secret string and an error if any.
func GetSecretFromOCIVault(ctx context.Context, secretsClient OCISecretsManagerAPI, secretOCID string, vaultRegion string) (string, error) {
	// Check if the passed secret OCID is empty
	if secretOCID == "" {
		return "", errors.New("secret OCID is empty")
	}

	// Check if the vault region is empty
	if vaultRegion == "" {
		return "", errors.New("vault region is empty")
	}

	// Set the region for the secrets client
	secretsClient.SetRegion(vaultRegion)

	// Create the request to get secret bundle
	getSecretBundleRequest := secrets.GetSecretBundleRequest{
		SecretId: ociCommon.String(secretOCID),
	}

	// Fetch the response from OCI Secrets Manager
	scResponse, err := secretsClient.GetSecretBundle(ctx, getSecretBundleRequest)
	if err != nil {
		return "", fmt.Errorf("failed to fetch secret bundle: %w", err)
	}
	log.Debug("successfully fetched secret from OCI vault")

	// Extract the secret content
	secretContent, ok := scResponse.SecretBundleContent.(secrets.Base64SecretBundleContentDetails)
	if !ok {
		log.WithField("secretOCID", secretOCID).Error("unexpected secret content type")
		return "", fmt.Errorf("unexpected secret content type")
	}

	if secretContent.Content == nil {
		log.WithField("secretOCID", secretOCID).Error("secret content is nil")
		return "", fmt.Errorf("secret content is nil")
	}

	decodedSecret, err := base64.StdEncoding.DecodeString(*secretContent.Content)
	if err != nil {
		log.WithField("error", err).WithField("secretOCID", secretOCID).Error("failed to base64 decode secret content")
		return "", fmt.Errorf("failed to decode secret content: %w", err)
	}

	return string(decodedSecret), nil
}

// NewOCISecretsManagerClient creates a new OCI Secrets Manager client.
// It returns an OCISecretsManagerAPI client and an error if any.
func NewOCISecretsManagerClient() (OCISecretsManagerAPI, error) {
	var provider ociCommon.ConfigurationProvider
	var err error

	provider, err = auth.ResourcePrincipalConfigurationProvider()
	if err != nil {
		log.WithField("error", err).Error("failed to create resource principal configuration provider")
		return nil, fmt.Errorf("failed to create resource principal configuration provider: %w", err)
	}

	secretsClient, err := secrets.NewSecretsClientWithConfigurationProvider(provider)
	if err != nil {
		log.WithField("error", err).Error("failed to create OCI secrets client")
		return nil, fmt.Errorf("failed to create OCI secrets client: %w", err)
	}

	return &secretsClient, nil
}

// GetLicenseKey returns the license key from the environment variable or the OCI Secrets Manager.
// It returns the New Relic Ingest License key and an error if any.
func GetLicenseKey() (key string, err error) {
	return GetLicenseKeyWithContext(context.Background())
}

// GetLicenseKeyWithContext returns the license key from the environment variable or the OCI Secrets Manager with context.
// It returns the New Relic Ingest License key and an error if any.
func GetLicenseKeyWithContext(ctx context.Context) (key string, err error) {
	// First try to get from environment variable
	if os.Getenv(common.EnvLicenseKey) != "" {
		log.Debug("fetching license key from environment variable")
		return os.Getenv(common.EnvLicenseKey), nil
	}

	log.Debug("fetching license key from OCI vault")

	// Get secret OCID and vault region from environment
	secretOCID := os.Getenv(common.SecretOCID)
	vaultRegion := os.Getenv(common.VaultRegion)

	// Create OCI secrets client
	secretsClient, err := NewOCISecretsManagerClient()
	if err != nil {
		return "", err
	}

	// Get the secret from OCI vault
	secretValue, err := GetSecretFromOCIVault(ctx, secretsClient, secretOCID, vaultRegion)
	if err != nil {
		return "", err
	}

	return secretValue, nil
}
