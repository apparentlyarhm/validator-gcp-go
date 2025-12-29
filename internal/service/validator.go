package service

import (
	"context"
	"net"
	"path"
	"slices"
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

	var o []option.ClientOption
	if cfg.GoogleCloud.ApplicationCredentials != "" {
		o = append(o, option.WithCredentialsFile(cfg.GoogleCloud.ApplicationCredentials))
	}
	// in production this wont be appended, that means it'll be inferred automatically

	// All of this can be thought of @Autowired equivalent
	fwClient := config.NewFirewallsClient(ctx, o...)
	instClient := config.NewInstancesClient(ctx, o...)
	storageClient := config.NewStorageClient(ctx, o...)
	mchTypeClient := config.NewMachinesTypeClient(ctx, o...)

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

/*
Returns pong
*/
func (s *ValidatorService) DoPong(ctx context.Context) *models.CommonResponse {
	res := &models.CommonResponse{
		Message: "Pong!",
	}

	return res
}

/*
Returns PRESENT or ABSENT if the IP is present in the sources list of the concerned firewall.
Returns PRESENT automatically if the list has `0.0.0.0/0`, signifying public access.
*/
func (s *ValidatorService) IsIpPresent(ctx context.Context, ip string) (*models.CommonResponse, error) {
	var message string = "ABSENT"

	source := net.ParseIP(ip)
	if source == nil {
		return nil, apperror.ErrBadRequest
	}

	var target = ip + "/32" // this method will only deal with single IPs
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	r := &computepb.GetFirewallRequest{
		Project:  s.cfg.GoogleCloud.Project,
		Firewall: s.cfg.GoogleCloud.FirewallName,
	}

	f, fe := s.firewallsClient.Get(ctx, r)
	if fe != nil {
		return nil, apperror.MapError(fe)
	}

	var ips = f.GetSourceRanges()
	if slices.Contains(ips, target) || slices.Contains(ips, "0.0.0.0/0") {
		message = "PRESENT"
	}

	return &models.CommonResponse{
		Message: message,
	}, nil

}
func (s *ValidatorService) AddIpToFirewall(request interface{}) {} // Use actual models later
func (s *ValidatorService) PurgeFirewall()                      {}
func (s *ValidatorService) AllowPublicAccess()                  {}
func (s *ValidatorService) GetFirewallDetails()                 {}
func (s *ValidatorService) GetServerInfo(address string)        {}
func (s *ValidatorService) GetModList()                         {}
func (s *ValidatorService) Download(filename string)            {}
func (s *ValidatorService) ExecuteRcon(address, command string) {}
