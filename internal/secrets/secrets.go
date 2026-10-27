package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// Secret constrains the types that can be stored in Secrets Manager.
type Secret interface {
	FigmaSecret | ShopifySecret
}

// SecretsClient wraps AWS Secrets Manager for storing integration credentials.
type SecretsClient struct {
	client *secretsmanager.Client
}

// NewSecretsClient creates a SecretsClient backed by the given Secrets Manager client.
func NewSecretsClient(client *secretsmanager.Client) *SecretsClient {
	return &SecretsClient{client: client}
}

// PutSecret marshals the value to JSON and creates or updates the named secret.
func PutSecret[T Secret](s *SecretsClient, ctx context.Context, name string, value T) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal secret: %w", err)
	}
	return s.createOrUpdate(ctx, name, string(data))
}

func (s *SecretsClient) createOrUpdate(ctx context.Context, name, value string) (string, error) {
	createOut, err := s.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: aws.String(value),
	})
	if err != nil {
		return s.updateExisting(ctx, name, value)
	}
	return aws.ToString(createOut.ARN), nil
}

func (s *SecretsClient) updateExisting(ctx context.Context, name, value string) (string, error) {
	putOut, err := s.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretString: aws.String(value),
	})
	if err != nil {
		return "", fmt.Errorf("put secret value: %w", err)
	}
	return aws.ToString(putOut.ARN), nil
}

// GetSecret fetches a secret by name and unmarshals it into the specified type.
func GetSecret[T Secret](s *SecretsClient, ctx context.Context, name string) (*T, error) {
	raw, err := s.fetchSecretString(ctx, name)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("unmarshal secret: %w", err)
	}
	return &out, nil
}

func (s *SecretsClient) fetchSecretString(ctx context.Context, name string) (string, error) {
	result, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})
	if err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}
	return aws.ToString(result.SecretString), nil
}

// Delete removes a secret from Secrets Manager.
func (s *SecretsClient) Delete(ctx context.Context, name string) error {
	_, err := s.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(name),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("delete secret %s: %w", name, err)
	}
	return nil
}

// ShopifySecretName returns the Secrets Manager name for Shopify credentials.
func ShopifySecretName(userID, storeID string) string {
	return fmt.Sprintf(ShopifySecretNameFmt, userID, storeID)
}

// FigmaSecretName returns the Secrets Manager name for Figma credentials.
func FigmaSecretName(userID, storeID string) string {
	return fmt.Sprintf(FigmaSecretNameFmt, userID, storeID)
}

// DeleteStoreSecrets removes both Shopify and Figma secrets for a store.
func DeleteStoreSecrets(s *SecretsClient, ctx context.Context, userID, storeID string) {
	if err := s.Delete(ctx, ShopifySecretName(userID, storeID)); err != nil {
		slog.Warn("failed to delete shopify secret", "storeId", storeID, "error", err)
	}
	if err := s.Delete(ctx, FigmaSecretName(userID, storeID)); err != nil {
		slog.Warn("failed to delete figma secret", "storeId", storeID, "error", err)
	}
}
