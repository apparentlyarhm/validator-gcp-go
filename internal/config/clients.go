package config

import (
	"context"
	"log"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func NewInstancesClient(ctx context.Context, o option.ClientOption) *compute.InstancesClient {
	c, e := compute.NewInstancesRESTClient(ctx, o)
	if e != nil {
		log.Fatalf("[INIT] Could not instantiate Instances Client :: %v", e)
	}

	log.Println(":: Instance client init ::")

	return c
}

func NewFirewallsClient(ctx context.Context, o option.ClientOption) *compute.FirewallsClient {
	c, e := compute.NewFirewallsRESTClient(ctx, o)
	if e != nil {
		log.Fatalf("[INIT] Could not instantiate Firewall Client :: %v", e)
	}

	log.Println(":: firewall client init ::")

	return c
}

func NewMachinesTypeClient(ctx context.Context, o option.ClientOption) *compute.MachineTypesClient {
	c, e := compute.NewMachineTypesRESTClient(ctx, o)
	if e != nil {
		log.Fatalf("[INIT] Could not instantiate Machine Type Client :: %v", e)
	}

	log.Println(":: MT client init ::")

	return c
}
func NewStorageClient(ctx context.Context, o option.ClientOption) *storage.Client {
	c, e := storage.NewClient(ctx, o)
	if e != nil {
		log.Fatalf("[INIT] Could not instantiate Storage Client :: %v", e)
	}

	log.Println(":: storage client init ::")

	return c
}
