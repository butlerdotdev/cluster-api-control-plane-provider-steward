# Steward Cluster API Control Plane Provider

<p align="left">
  <a href="https://github.com/butlerdotdev/cluster-api-control-plane-provider-steward/blob/master/LICENSE"><img src="https://img.shields.io/github/license/butlerdotdev/cluster-api-control-plane-provider-steward" alt="License"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/butlerdotdev/cluster-api-control-plane-provider-steward" alt="Go Version">
  <a href="https://goreportcard.com/report/github.com/butlerdotdev/cluster-api-control-plane-provider-steward"><img src="https://goreportcard.com/badge/github.com/butlerdotdev/cluster-api-control-plane-provider-steward" alt="Go Report Card"></a>
  <a href="https://github.com/butlerdotdev/cluster-api-control-plane-provider-steward/releases"><img src="https://img.shields.io/github/v/release/butlerdotdev/cluster-api-control-plane-provider-steward" alt="Release"></a>
</p>

The Steward Control Plane Provider is a [Cluster API](https://cluster-api.sigs.k8s.io/) implementation that bridges CAPI with [Steward](https://github.com/butlerdotdev/steward) hosted control planes.

## What is This?

This provider enables Cluster API to use Steward's TenantControlPlane resources as the control plane for CAPI-managed clusters. Instead of provisioning dedicated control plane nodes, the control plane runs as pods in a management cluster, managed by Steward.

**How it works:**

1. CAPI creates a `Cluster` resource referencing a `StewardControlPlane`
2. This provider creates a Steward `TenantControlPlane` resource
3. Steward provisions the hosted control plane (apiserver, controller-manager, scheduler as pods)
4. Worker nodes from any CAPI infrastructure provider join the hosted control plane
5. The provider synchronizes status between CAPI and Steward

## What is Steward?

[Steward](https://github.com/butlerdotdev/steward) is an open-source project offering hosted Kubernetes control planes. The control plane runs in a management cluster as regular pods, enabling efficient multi-tenancy and reduced infrastructure overhead.

Steward is a community-governed fork of Kamaji, maintained by Butler Labs and the open source community. See the [Steward documentation](https://docs.butlerlabs.dev/steward/) for more information.

## StewardControlPlane Example

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
kind: StewardControlPlane
metadata:
  name: my-cluster-control-plane
  namespace: default
spec:
  version: "1.29.0"
  replicas: 2
  dataStoreName: default
  network:
    serviceType: LoadBalancer
    certSANs:
      - "my-cluster.example.com"
  kubelet:
    preferredAddressTypes:
      - InternalIP
      - ExternalIP
      - Hostname
    cgroupfs: systemd
  addons:
    coreDNS: {}
    kubeProxy: {}
```

## CAPI Cluster Example

Complete example using StewardControlPlane with a CAPI infrastructure provider:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
        - 10.244.0.0/16
    services:
      cidrBlocks:
        - 10.96.0.0/16
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
    kind: StewardControlPlane
    name: my-cluster-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: <InfrastructureCluster>
    name: my-cluster
---
apiVersion: controlplane.cluster.x-k8s.io/v1alpha1
kind: StewardControlPlane
metadata:
  name: my-cluster-control-plane
  namespace: default
spec:
  version: "1.29.0"
  replicas: 2
  dataStoreName: default
  network:
    serviceType: LoadBalancer
  addons:
    coreDNS: {}
    kubeProxy: {}
```

## Supported CAPI Infrastructure Providers

| Infrastructure Provider | Version | Notes |
|------------------------|---------|-------|
| [AWS](https://github.com/kubernetes-sigs/cluster-api-provider-aws) | >= v2.4.0 | [Technical considerations](docs/providers-aws.md) |
| [Azure](https://github.com/kubernetes-sigs/cluster-api-provider-azure) | >= v1.18.0 | [Technical considerations](docs/providers-azure.md) |
| [Equinix/Packet](https://github.com/kubernetes-sigs/cluster-api-provider-packet) | >= v0.7.2 | [Technical considerations](docs/providers-packet.md) |
| [Hetzner](https://github.com/syself/cluster-api-provider-hetzner) | >= v1.0.0-beta.30 | [Technical considerations](docs/providers-hetzner.md) |
| [IONOS Cloud](https://github.com/ionos-cloud/cluster-api-provider-ionoscloud) | >= v0.3.0 | [Technical considerations](docs/providers-ionoscloud.md) |
| [KubeVirt](https://github.com/kubernetes-sigs/cluster-api-provider-kubevirt) | >= 0.1.7 | [Technical considerations](docs/providers-kubevirt.md) |
| [Metal3](https://github.com/metal3-io/cluster-api-provider-metal3) | >= 1.4.0 | [Technical considerations](docs/providers-metal3.md) |
| [Nutanix](https://github.com/nutanix-cloud-native/cluster-api-provider-nutanix) | >= 1.2.4 | [Technical considerations](docs/providers-nutanix.md) |
| [OpenStack](https://github.com/kubernetes-sigs/cluster-api-provider-openstack) | >= 0.8.0 | [Technical considerations](docs/providers-openstack.md) |
| [Proxmox](https://github.com/ionos-cloud/cluster-api-provider-proxmox) | >= v0.6.0 | [Technical considerations](docs/providers-proxmox.md) |
| [Tinkerbell](https://github.com/tinkerbell/cluster-api-provider-tinkerbell) | >= v0.5.2 | [Technical considerations](docs/providers-tinkerbell.md) |
| [vSphere](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere) | >= 1.7.0 | [Technical considerations](docs/providers-vsphere.md) |

Looking for additional integrations? Open a [GitHub Discussion](https://github.com/butlerdotdev/cluster-api-control-plane-provider-steward/discussions) or [issue](https://github.com/butlerdotdev/cluster-api-control-plane-provider-steward/issues).

## Prerequisites

- Kubernetes management cluster (v1.28+)
- [Steward](https://github.com/butlerdotdev/steward) installed and configured with a DataStore
- [Cluster API](https://cluster-api.sigs.k8s.io/) core components (v1.6+)
- A supported CAPI infrastructure provider

## Installation

### Using clusterctl

```bash
clusterctl init --control-plane steward
```

### Using Helm

```bash
helm repo add butler https://charts.butlerlabs.dev
helm install capi-steward butler/capi-steward -n capi-system
```

## Development

This document describes how to use kind and [Tilt](https://tilt.dev/) for a simplified workflow that offers easy deployments and rapid iterative builds.

1. Create a `kind` cluster according to the [CAPI Infrastructure Provider requirements](https://cluster-api.sigs.k8s.io/user/quick-start#install-andor-configure-a-kubernetes-cluster)
2. [Install Cluster API](https://cluster-api.sigs.k8s.io/user/quick-start#initialize-the-management-cluster) with the `clusterctl` CLI
3. Install [Steward](https://github.com/butlerdotdev/steward) using Helm
4. Clone this repository
5. Run the provider with `make run` or use `dlv` for debugging
6. Run Tilt by issuing `tilt up`

## Versioning

Versioning adheres to [Semantic Versioning](http://semver.org/) principles. A full list of releases is available in the [Releases](https://github.com/butlerdotdev/cluster-api-control-plane-provider-steward/releases) section.

## Contributing

Contributions are welcome! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

- Check existing [issues](https://github.com/butlerdotdev/cluster-api-control-plane-provider-steward/issues) before opening a new one
- For bugs, provide a detailed report to help replicate and assess the issue
- Commit messages follow [conventional commits](https://www.conventionalcommits.org/)

## Documentation

- [Steward Documentation](https://docs.butlerlabs.dev/steward/)
- [Cluster API Documentation](https://cluster-api.sigs.k8s.io/)
- [Provider Technical Considerations](docs/)

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
