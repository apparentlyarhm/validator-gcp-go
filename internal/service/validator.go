package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"path"
	"slices"
	"strconv"
	"strings"
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

/*
Returns a minimal info about the firewall. at the time of writing this, its not really used anywhere.
*/
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

/*
Reads the modlist.txt on the associated bucket, parses its contents and file's updated timestamp
to return to the frontend. ".jar" substring is stripped from all file names.
*/
func (s *ValidatorService) GetModList(ctx context.Context) (*models.ModListResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var mods []string
	var updated string

	bkt := s.storageClient.Bucket(s.cfg.GoogleCloud.BucketName)
	o := bkt.Object(s.cfg.GoogleCloud.ModlistFile)

	oAtrs, err := o.Attrs(ctx)
	if err != nil {
		return nil, apperror.MapError(err)
	}

	updated = oAtrs.Updated.String()

	r, rerr := o.NewReader(ctx)
	if rerr != nil {
		return nil, apperror.MapError(rerr) // default behaviour is log and return 500 so this is fine i think.
	}
	defer r.Close()

	modlistBytes, readError := io.ReadAll(r)
	if readError != nil {
		return nil, apperror.MapError(readError)
	}

	mlstr := strings.SplitSeq(string(modlistBytes), "\n")
	for line := range mlstr {
		mods = append(mods, strings.Split(path.Base(line), ".jar")[0]) // file1.jar\n -> file1.jar -> file1
	}

	return &models.ModListResponse{
		Mods:      mods,
		UpdatedAt: updated,
	}, nil
}

/*
Signs a file on the associated bucket for 5 minutes and returns its download link.
returns bad request in case file isnt found. (in normal user flow it shouldnt be possible)
*/
func (s *ValidatorService) Download(ctx context.Context, filename string) (*models.CommonResponse, error) {
	blobPath := fmt.Sprintf("files/%s.jar", filename)
	objHandle := s.storageClient.Bucket(s.cfg.GoogleCloud.BucketName).Object(blobPath)

	log.Printf(":: %v ::", blobPath)

	_, err := objHandle.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, apperror.ErrNotFound
		}
		return nil, apperror.MapError(err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	/*
		as per the docs:
		"Detecting GoogleAccessID may not be possible if you are authenticated using a token source or using option.WithHTTPClient.
		In this case, you can provide a service account email for GoogleAccessID and the client will attempt to sign the URL or
		Post Policy using that service account."

		`Cloud Run (via Application Default Credentials) uses a Token Source (it fetches access tokens from the metadata server).
		It does not provide the private key file.

		Because of this, the Go library often cannot "guess" which Service Account it is running as, so it can't ask the IAM API
		to sign the blob. Java likely does an extra network call behind the scenes to "get current identity" that Go avoids for
		performance.`

		^^ this is what gemini had to say for this, and makes sense.

	*/

	opts := &storage.SignedURLOptions{
		Scheme:  storage.SigningSchemeV4,
		Method:  "GET",
		Expires: time.Now().Add(5 * time.Minute),

		// If we define this, the library skips the "guessing" phase.
		// It will immediately use the IAM Credentials API to sign the URL remotely
		GoogleAccessID: s.cfg.GoogleCloud.ServiceAccountEmail,
	}

	url, err := s.storageClient.Bucket(s.cfg.GoogleCloud.BucketName).SignedURL(blobPath, opts)
	if err != nil {
		return nil, apperror.MapError(err)
	}

	/*
		Local: If you use a JSON key file, the library ignores GoogleAccessID and signs locally using the key in the file. It still works.

		Cloud Run: It sees the GoogleAccessID, realizes it doesn't have a local private key, and automatically calls the iam.serviceAccounts.signBlob
		API using the ADC tokens. It works.
	*/
	return &models.CommonResponse{
		Message: url,
	}, nil

}

func (s *ValidatorService) GetServerInfo(ip string)        {}
func (s *ValidatorService) ExecuteRcon(ip, command string) {}

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
