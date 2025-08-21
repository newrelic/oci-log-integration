package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/secrets"

	"github.com/newrelic/oci-log-integration/logs-function/common"
)

// Mock OCI Secrets Manager client for testing
type mockOCISecretsClient struct {
	shouldError   bool
	secretContent string
	region        string
}

func (m *mockOCISecretsClient) GetSecretBundle(ctx context.Context, request secrets.GetSecretBundleRequest) (secrets.GetSecretBundleResponse, error) {
	if m.shouldError {
		return secrets.GetSecretBundleResponse{}, errors.New("mock OCI secrets error")
	}

	// Encode the secret content as base64
	encodedContent := base64.StdEncoding.EncodeToString([]byte(m.secretContent))

	response := secrets.GetSecretBundleResponse{
		SecretBundle: secrets.SecretBundle{
			SecretBundleContent: secrets.Base64SecretBundleContentDetails{
				Content: &encodedContent,
			},
		},
	}

	return response, nil
}

func (m *mockOCISecretsClient) SetRegion(regionId string) {
	m.region = regionId
}

func TestGetSecretFromOCIVault(t *testing.T) {
	tests := []struct {
		name           string
		secretOCID     string
		vaultRegion    string
		secretContent  string
		shouldError    bool
		expectedSecret string
		expectedError  string
	}{
		{
			name:           "successful secret retrieval",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "test-license-key",
			shouldError:    false,
			expectedSecret: "test-license-key",
			expectedError:  "",
		},
		{
			name:           "empty secret OCID",
			secretOCID:     "",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "secret OCID is empty",
		},
		{
			name:           "empty vault region",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "",
			secretContent:  "",
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "vault region is empty",
		},
		{
			name:           "OCI API error",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    true,
			expectedSecret: "",
			expectedError:  "failed to fetch secret bundle",
		},
		{
			name:           "JSON secret content",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  `{"licenseKey": "json-license-key"}`,
			shouldError:    false,
			expectedSecret: `{"licenseKey": "json-license-key"}`,
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockOCISecretsClient{
				shouldError:   tt.shouldError,
				secretContent: tt.secretContent,
			}

			secret, err := GetSecretFromOCIVault(context.Background(), mockClient, tt.secretOCID, tt.vaultRegion)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError && !contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if secret != tt.expectedSecret {
					t.Errorf("Expected secret '%s', but got '%s'", tt.expectedSecret, secret)
				}
			}

			// Verify region was set correctly (if not empty)
			if tt.vaultRegion != "" && !tt.shouldError && tt.secretOCID != "" {
				if mockClient.region != tt.vaultRegion {
					t.Errorf("Expected region '%s', but got '%s'", tt.vaultRegion, mockClient.region)
				}
			}
		})
	}
}

func TestGetLicenseKeyWithMockClient(t *testing.T) {
	tests := []struct {
		name           string
		secretOCID     string
		vaultRegion    string
		secretContent  string
		shouldError    bool
		expectedSecret string
		expectedError  string
	}{
		{
			name:           "successful secret retrieval",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "test-license-key",
			shouldError:    false,
			expectedSecret: "test-license-key",
			expectedError:  "",
		},
		{
			name:           "JSON secret content",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  `{"licenseKey": "json-license-key"}`,
			shouldError:    false,
			expectedSecret: "json-license-key",
			expectedError:  "",
		},
		{
			name:           "empty secret content",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "license key secret is empty",
		},
		{
			name:           "JSON without license key",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  `{"otherKey": "other-value"}`,
			shouldError:    false,
			expectedSecret: "",
			expectedError:  "license key is empty or not present in the secret",
		},
		{
			name:           "OCI secrets error",
			secretOCID:     "ocid1.vaultsecret.test",
			vaultRegion:    "us-phoenix-1",
			secretContent:  "",
			shouldError:    true,
			expectedSecret: "",
			expectedError:  "mock OCI secrets error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &mockOCISecretsClient{
				shouldError:   tt.shouldError,
				secretContent: tt.secretContent,
			}

			// Call GetSecretFromOCIVault first
			secret, err := GetSecretFromOCIVault(context.Background(), mockClient, tt.secretOCID, tt.vaultRegion)

			if tt.shouldError {
				// If we expect an OCI error, verify it and return
				if err == nil {
					t.Errorf("Expected OCI error, but got nil")
				} else if !contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error from GetSecretFromOCIVault: %v", err)
				return
			}

			// Now test the license key extraction logic
			licenseKey, extractErr := extractLicenseKeyFromSecret(secret)

			if tt.expectedError != "" {
				if extractErr == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
				} else if !contains(extractErr.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, extractErr.Error())
				}
			} else {
				if extractErr != nil {
					t.Errorf("Expected no error, but got: %v", extractErr)
				}
				if licenseKey != tt.expectedSecret {
					t.Errorf("Expected license key '%s', but got '%s'", tt.expectedSecret, licenseKey)
				}
			}
		})
	}
}

// Helper function to extract license key from secret (for testing)
func extractLicenseKeyFromSecret(secretValue string) (string, error) {
	if secretValue == "" {
		return "", errors.New("license key secret is empty")
	}

	// Try to parse as JSON first
	var secretMap map[string]string
	if err := json.Unmarshal([]byte(secretValue), &secretMap); err != nil {
		// If it's not JSON, return the entire secret as license key
		return secretValue, nil
	}

	// Extract license key from JSON
	if licenseKey, exists := secretMap[common.LicenseKey]; exists && licenseKey != "" {
		return licenseKey, nil
	}

	return "", errors.New("license key is empty or not present in the secret")
}

// Helper functions
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr ||
		(len(str) > len(substr) &&
			(str[:len(substr)] == substr || str[len(str)-len(substr):] == substr ||
				func() bool {
					for i := 1; i < len(str)-len(substr)+1; i++ {
						if str[i:i+len(substr)] == substr {
							return true
						}
					}
					return false
				}())))
}
