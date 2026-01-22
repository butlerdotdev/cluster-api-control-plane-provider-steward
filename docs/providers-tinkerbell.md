# Steward and Tinkerbell

The Steward Control Plane provider allows creating a _Tinkerbell_ backed Kubernetes cluster by providing Steward Control Planes.

## Example manifests

> To be provided.

## Technical considerations

The Steward Control Plane provider is in charge of creating, thanks to Steward, a Load Balancer for the API Server.

[Despite following the official documentation](https://github.com/tinkerbell/cluster-api-provider-tinkerbell/blob/main/docs/QUICK-START.md#required-configuration-for-the-tinkerbell-provider),
you can set an empty value for the Control Plane VIP, since the Steward Control Plane provider will patch it.

> Remember that Steward supports pre-allocated addresses, in such case the two VIPs must match:
> otherwise, the Cluster API reconciliation will fail and blocked.
