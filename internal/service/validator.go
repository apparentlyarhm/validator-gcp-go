package service

import (
	"context"
	"log"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/storage"
	"github.com/validator-gcp/v2/internal/config"
)

type ValidatorService struct {
	cfg *config.Config

	// clients
	firewallsClient   *compute.FirewallsClient
	instancesClient   *compute.InstancesClient
	machineTypeClient *compute.MachineTypesClient
	storageClient     *storage.Client
}

func NewValidatorService(cfg *config.Config) (*ValidatorService, error) {
	ctx := context.Background()

	// All of this can be thought of @Autowired equivalent
	fwClient, err := config.NewFirewallsClient(ctx)
	if err != nil {
		log.Fatalf("[INIT]: Could not start up Firewall Client")
	}

	instClient, err := config.NewInstancesClient(ctx)
	if err != nil {
		log.Fatalf("[INIT]: Could not start up Instances Client")
	}

	storageClient, err := config.NewStorageClient(ctx)
	if err != nil {
		log.Fatalf("[INIT]: Could not start up Storage Client")
	}

	mchTypeClient, err := config.NewMachinesTypeClient(ctx)
	if err != nil {
		log.Fatalf("[INIT]: Could not start up Machine Types Client")
	}

	return &ValidatorService{
		cfg:               cfg,
		firewallsClient:   fwClient,
		instancesClient:   instClient,
		storageClient:     storageClient,
		machineTypeClient: mchTypeClient,
	}, nil
}
