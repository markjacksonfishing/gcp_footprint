# GCP Footprint

A comprehensive Go script that connects to your GCP account and retrieves extensive information about various GCP resources across all regions, saving the data to a text file.

## Overview

GCP Footprint is a tool designed to help you quickly inventory and audit your Google Cloud Platform resources. It queries multiple GCP services across all available regions and generates a detailed report of your cloud infrastructure.

## Features

- **Comprehensive Resource Discovery**: Queries a wide range of GCP services
- **Multi-Region Support**: Automatically scans all GCP regions
- **Detailed Output**: Generates a structured text file with resource information
- **Easy Authentication**: Supports multiple authentication methods
- **Containerized Deployment**: Includes Docker and Kubernetes support

## Resources Queried

### Global Resources
- Projects
- Storage Buckets
- IAM Roles and Bindings
- Service Accounts
- Firewall Rules
- Snapshots

### Regional Resources
- Compute Engine Instances
- Google Kubernetes Engine (GKE) Clusters
- Cloud SQL Instances
- VPC Networks
- Subnets
- Persistent Disks

## Prerequisites

- Go 1.21 or later
- GCP account with appropriate permissions
- One of the following authentication methods:
  - Service Account JSON key file
  - Application Default Credentials (ADC)
  - Google Cloud SDK authenticated session

## Installation

### From Source

```bash
git clone https://github.com/markyjacksonfishing/gcp_footprint.git
cd gcp_footprint
go mod download
go build -o gcp_footprint
```

### Using Docker

```bash
docker build -t gcp_footprint .
```

## Usage

### Local Execution

1. Set up authentication (choose one method):

   **Option A: Service Account Key**
   ```bash
   export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account-key.json"
   ```

   **Option B: Use gcloud auth**
   ```bash
   gcloud auth application-default login
   ```

2. Run the tool:
   ```bash
   ./gcp_footprint
   ```

3. Enter your GCP Project ID when prompted

4. The tool will generate a file named `gcp_footprint_<project-id>.txt`

### Docker Execution

```bash
# With service account key
docker run -it -v /path/to/service-account-key.json:/creds/key.json \
  -e GOOGLE_APPLICATION_CREDENTIALS=/creds/key.json \
  -v $(pwd):/output \
  gcp_footprint

# With ADC credentials
docker run -it -v ~/.config/gcloud:/root/.config/gcloud \
  -v $(pwd):/output \
  gcp_footprint
```

### Kubernetes Deployment

1. Create a secret with your service account key:
   ```bash
   kubectl create secret generic gcp-creds \
     --from-file=key.json=/path/to/service-account-key.json
   ```

2. Deploy the application:
   ```bash
   kubectl apply -f k8s-deployment.yaml
   kubectl apply -f k8s-service.yaml
   ```

## Required GCP Permissions

The service account or user running this tool needs the following roles:
- `roles/viewer` (Project Viewer)
- `roles/resourcemanager.organizationViewer` (if querying organization resources)
- `roles/iam.securityReviewer` (for detailed IAM information)

Or these specific permissions:
- `compute.instances.list`
- `compute.networks.list`
- `compute.subnetworks.list`
- `compute.firewalls.list`
- `compute.disks.list`
- `compute.snapshots.list`
- `container.clusters.list`
- `cloudsql.instances.list`
- `storage.buckets.list`
- `iam.serviceAccounts.list`
- `resourcemanager.projects.get`
- `resourcemanager.projects.getIamPolicy`

## Output Format

The tool generates a structured text file with the following sections:

```
GCP FOOTPRINT REPORT
====================
Generated: 2024-01-15 10:30:45
Project ID: my-project-123

PROJECT INFORMATION
==================
[Project]
Name: My Project
Project ID: my-project-123
...

GLOBAL RESOURCES
===============
[Storage Bucket]
Name: my-bucket
Location: US
...

REGION: us-central1
==================
[Compute Instance]
Name: web-server-1
Machine Type: e2-medium
...
```

## Extending the Tool

To add support for additional GCP services:

1. Add the necessary client library to `go.mod`
2. Create a new function in `gcp_footprint.go` following the pattern:
   ```go
   func getResourceType(ctx context.Context, region string) {
       // Implementation
   }
   ```
3. Call your function from the appropriate section (global or regional)

## Security Considerations

- Never commit service account keys to version control
- Use least-privilege service accounts
- Rotate credentials regularly
- Consider using Workload Identity for GKE deployments

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

Inspired by the [aws_footprint](https://github.com/markjacksonfishing/aws_footprint) project.
