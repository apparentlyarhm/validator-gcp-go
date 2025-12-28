package config

import (
	"context"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/storage"
)

func NewInstancesClient(ctx context.Context) (*compute.InstancesClient, error) {
	// TODO: Initialize and return GCP InstancesClient

	return nil, nil
}

func NewFirewallsClient(ctx context.Context) (*compute.FirewallsClient, error) {

	return nil, nil
}

func NewMachinesTypeClient(ctx context.Context) (*compute.MachineTypesClient, error) {

	return nil, nil
}

func NewStorageClient(ctx context.Context) (*storage.Client, error) {

	return nil, nil
}
