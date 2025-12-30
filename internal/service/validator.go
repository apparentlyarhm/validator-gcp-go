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

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second) // fair timeout?
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

	source := parseIP(ip)
	if source == nil {
		return nil, apperror.ErrBadRequest
	}

	var target = ip + "/32" // this method will only deal with single IPs
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
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

/*
Adds a given ip to the related firewall's Sources List. Request is rejected if
ip doesnt successful parse as `net.IP`.
*/
func (s *ValidatorService) AddIpToFirewall(ctx context.Context, ip string) (*models.CommonResponse, error) {
	source := parseIP(ip)
	if source == nil {
		return nil, apperror.ErrBadRequest
	}

	var target = ip + "/32" // this method will only deal with single IPs
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
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
	if slices.Contains(ips, target) {
		return nil, apperror.ErrConflict
	}

	// matching the logic from the original app
	if len(ips) > 50 {
		ips = []string{} // new empty
	}

	ips = append(ips, target) // our new ip list is complete at this point in any case.

	patchReq := &computepb.PatchFirewallRequest{
		Project:  s.cfg.GoogleCloud.Project,
		Firewall: s.cfg.GoogleCloud.FirewallName,
		FirewallResource: &computepb.Firewall{
			SourceRanges: ips,
		},
	}

	// The Go client returns an "Operation" object, just like Java's Future/Operation.
	op, err := s.firewallsClient.Patch(ctx, patchReq)
	if err != nil {
		return nil, apperror.MapError(err)
	}

	if err = op.Wait(ctx); err != nil {
		return nil, apperror.MapError(err)
	}

	return &models.CommonResponse{
		Message: "IP added to firewall successfully",
	}, nil

}

/*
Removes all IPs from the firewall and adds a dummy - 1.1.1.1/32,
effectively preventing public access to resources until ips are populated back in
*/
func (s *ValidatorService) PurgeFirewall(ctx context.Context) (*models.CommonResponse, error) {
	var target = "1.1.1.1" + "/32"
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var ips []string
	ips = append(ips, target)

	patchReq := &computepb.PatchFirewallRequest{
		Project:  s.cfg.GoogleCloud.Project,
		Firewall: s.cfg.GoogleCloud.FirewallName,
		FirewallResource: &computepb.Firewall{
			SourceRanges: ips,
		},
	}

	op, err := s.firewallsClient.Patch(ctx, patchReq)
	if err != nil {
		return nil, apperror.MapError(err)
	}

	if err = op.Wait(ctx); err != nil {
		return nil, apperror.MapError(err)
	}

	return &models.CommonResponse{
		Message: "Done",
	}, nil
}

/*
Removes all IPs from the firewall and adds 0.0.0.0/0
effectively allowing public access to resources (minecraft server)
*/
func (s *ValidatorService) AllowPublicAccess(ctx context.Context) (*models.CommonResponse, error) {
	var target = "0.0.0.0" + "/0"
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var ips []string
	ips = append(ips, target)

	patchReq := &computepb.PatchFirewallRequest{
		Project:  s.cfg.GoogleCloud.Project,
		Firewall: s.cfg.GoogleCloud.FirewallName,
		FirewallResource: &computepb.Firewall{
			SourceRanges: ips,
		},
	}

	op, err := s.firewallsClient.Patch(ctx, patchReq)
	if err != nil {
		return nil, apperror.MapError(err)
	}

	if err = op.Wait(ctx); err != nil {
		return nil, apperror.MapError(err)
	}

	return &models.CommonResponse{
		Message: "Done",
	}, nil
}

func (s *ValidatorService) GetFirewallDetails(ctx context.Context) (*models.FirwallRuleResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	r := &computepb.GetFirewallRequest{
		Project:  s.cfg.GoogleCloud.Project,
		Firewall: s.cfg.GoogleCloud.FirewallName,
	}

	f, fe := s.firewallsClient.Get(ctx, r)
	if fe != nil {
		return nil, apperror.MapError(fe)
	}

	var status string = "ENABLED"
	if f.GetDisabled() {
		status = "DISABLED"
	}

	return &models.FirwallRuleResponse{
		Name:         f.GetName(),
		Status:       status,
		Direction:    f.GetDirection(),
		AddressCount: len(f.GetSourceRanges()),
	}, nil

}
func (s *ValidatorService) GetServerInfo(ip string)        {}
func (s *ValidatorService) GetModList()                    {}
func (s *ValidatorService) Download(filename string)       {}
func (s *ValidatorService) ExecuteRcon(ip, command string) {}

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
