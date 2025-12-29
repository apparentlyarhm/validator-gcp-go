package service

import (
	"context"
	"path"
	"strconv"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/storage"
	"github.com/validator-gcp/v2/internal/apperror"
	"github.com/validator-gcp/v2/internal/config"
	"github.com/validator-gcp/v2/internal/models"
	"google.golang.org/api/option"
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
	o := option.WithCredentialsFile(cfg.GoogleCloud.ApplicationCredentials)

	// All of this can be thought of @Autowired equivalent
	fwClient := config.NewFirewallsClient(ctx, o)
	instClient := config.NewInstancesClient(ctx, o)
	storageClient := config.NewStorageClient(ctx, o)
	mchTypeClient := config.NewMachinesTypeClient(ctx, o)

	return &ValidatorService{
		cfg:               cfg,
		firewallsClient:   fwClient,
		instancesClient:   instClient,
		storageClient:     storageClient,
		machineTypeClient: mchTypeClient,
	}, nil
}

/*
Returns the detailed config of the VM running the minecraft server.
*/
func (s *ValidatorService) GetMachineDetails(ctx context.Context) (*models.InstanceDetailResponse, error) {

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second) // fair timeout?
	defer cancel()

	r := &computepb.GetInstanceRequest{
		Project:  s.cfg.GoogleCloud.Project,
		Instance: s.cfg.GoogleCloud.VMName,
		Zone:     s.cfg.GoogleCloud.VMZone,
	}

	i, ie := s.instancesClient.Get(ctx, r)
	if ie != nil {
		return nil, apperror.MapError(ie)
	}

	var publicIp string
	for _, nwInterface := range i.GetNetworkInterfaces() {

		if len(nwInterface.GetAccessConfigs()) > 0 {
			publicIp = nwInterface.GetAccessConfigs()[0].GetNatIP()
			break
		}
	}

	var mtName = path.Base(i.GetMachineType())

	mtR := &computepb.GetMachineTypeRequest{
		Project:     s.cfg.GoogleCloud.Project,
		Zone:        s.cfg.GoogleCloud.VMZone,
		MachineType: mtName,
	}

	mt, mte := s.machineTypeClient.Get(ctx, mtR)
	if mte != nil {
		return nil, apperror.MapError(mte)
	}

	var diskSize int64
	if len(i.GetDisks()) > 0 {
		diskSize = i.GetDisks()[0].GetDiskSizeGb()
	}

	// use the Getters (e.g., GetName()) instead of *i.Name for nil-safety.
	res := &models.InstanceDetailResponse{

		InstanceName:      i.GetName(),
		InstanceZone:      path.Base(i.GetZone()),
		MachineType:       mt.GetName(),
		InstanceId:        strconv.FormatUint(i.GetId(), 10),
		Status:            i.GetStatus(),
		CreationTimestamp: i.GetCreationTimestamp(),
		PublicIp:          publicIp,
		CpuCores:          int(mt.GetGuestCpus()),
		MemoryMb:          int(mt.GetMemoryMb()),
		DiskGb:            int32(diskSize),
	}

	return res, nil
}

// TODO: implement these
func (v *ValidatorService) DoPong()                             {}
func (v *ValidatorService) AddIpToFirewall(request interface{}) {} // Use actual models later
func (v *ValidatorService) IsIpPresent(ip string)               {}
func (v *ValidatorService) PurgeFirewall()                      {}
func (v *ValidatorService) AllowPublicAccess()                  {}
func (v *ValidatorService) GetFirewallDetails()                 {}
func (v *ValidatorService) GetServerInfo(address string)        {}
func (v *ValidatorService) GetModList()                         {}
func (v *ValidatorService) Download(object string)              {}
func (v *ValidatorService) ExecuteRcon(address, command string) {}
