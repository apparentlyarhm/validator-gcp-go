package service

import (
	"context"
	"log/slog"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/storage"
	"github.com/validator-gcp/v2/internal/config"
)

type ValidatorService struct {
	cfg *config.Config

	// clients
	firewallsClient *compute.FirewallsClient
	instancesClient *compute.InstancesClient
	storageClient   *storage.Client

	logger *slog.Logger
}

func NewValidatorService(cfg *config.Config, l *slog.Logger) (*ValidatorService, error) {
	ctx := context.Background()

	fwClient, err := config.NewFirewallsClient(ctx)
	if err != nil {
		return nil, err
	}

	instClient, err := config.NewInstancesClient(ctx)
	if err != nil {
		return nil, err
	}

	storageClient, err := config.NewStorageClient(ctx)
	if err != nil {
		return nil, err
	}

	return &ValidatorService{
		cfg:             cfg,
		firewallsClient: fwClient,
		instancesClient: instClient,
		storageClient:   storageClient,
		logger:          l,
	}, nil
}
