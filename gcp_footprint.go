package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	container "cloud.google.com/go/container/apiv1"
	"cloud.google.com/go/container/apiv1/containerpb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/iterator"
	sqladmin "google.golang.org/api/sqladmin/v1"
)

var (
	outputFile *os.File
	projectID  string
	regions    = []string{
		"us-central1", "us-east1", "us-east4", "us-west1", "us-west2", "us-west3", "us-west4",
		"europe-west1", "europe-west2", "europe-west3", "europe-west4", "europe-west6",
		"europe-north1", "europe-central2",
		"asia-east1", "asia-east2", "asia-northeast1", "asia-northeast2", "asia-northeast3",
		"asia-south1", "asia-south2", "asia-southeast1", "asia-southeast2",
		"australia-southeast1", "australia-southeast2",
		"northamerica-northeast1", "northamerica-northeast2",
		"southamerica-east1", "southamerica-west1",
		"me-west1", "me-central1",
		"africa-south1",
	}
)

func main() {
	fmt.Println("GCP Footprint Tool")
	fmt.Println("==================")

	// Get project ID from user
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter GCP Project ID: ")
	projectID, _ = reader.ReadString('\n')
	projectID = strings.TrimSpace(projectID)

	// Check for credentials
	credsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credsFile == "" {
		fmt.Println("\nNo GOOGLE_APPLICATION_CREDENTIALS environment variable found.")
		fmt.Print("Enter path to service account key JSON file (or press Enter to use default credentials): ")
		credsPath, _ := reader.ReadString('\n')
		credsPath = strings.TrimSpace(credsPath)
		if credsPath != "" {
			os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credsPath)
		}
	}

	// Create output file
	fileName := fmt.Sprintf("gcp_footprint_%s.txt", projectID)
	var err error
	outputFile, err = os.Create(fileName)
	if err != nil {
		log.Fatalf("Failed to create output file: %v", err)
	}
	defer outputFile.Close()

	writeHeader()

	ctx := context.Background()

	// Get project information
	getProjectInfo(ctx)

	// Global resources
	fmt.Println("\nQuerying global resources...")
	writeSection("GLOBAL RESOURCES")

	getStorageBuckets(ctx)
	getIAMRoles(ctx)
	getServiceAccounts(ctx)

	// Regional resources
	fmt.Println("\nQuerying regional resources...")
	for _, region := range regions {
		fmt.Printf("\nChecking region: %s\n", region)
		writeSection(fmt.Sprintf("REGION: %s", region))

		getComputeInstances(ctx, region)
		getGKEClusters(ctx, region)
		getCloudSQLInstances(ctx, region)
		getVPCs(ctx, region)
		getSubnets(ctx, region)
		getDisks(ctx, region)
	}

	// Global resources that should only be queried once
	writeSection("GLOBAL FIREWALL RULES")
	getFirewallRules(ctx)
	
	writeSection("GLOBAL SNAPSHOTS")
	getSnapshots(ctx)

	fmt.Printf("\n\nGCP footprint saved to: %s\n", fileName)
}

func writeHeader() {
	header := fmt.Sprintf(`GCP FOOTPRINT REPORT
====================
Generated: %s
Project ID: %s

This report contains information about GCP resources in your project.
`, time.Now().Format("2006-01-02 15:04:05"), projectID)

	_, err := outputFile.WriteString(header)
	if err != nil {
		log.Printf("Failed to write header: %v", err)
	}
}

func writeSection(title string) {
	section := fmt.Sprintf("\n\n%s\n%s\n", title, strings.Repeat("=", len(title)))
	_, err := outputFile.WriteString(section)
	if err != nil {
		log.Printf("Failed to write section: %v", err)
	}
}

func writeResource(resourceType, info string) {
	_, err := outputFile.WriteString(fmt.Sprintf("\n[%s]\n%s\n", resourceType, info))
	if err != nil {
		log.Printf("Failed to write resource: %v", err)
	}
}

func getProjectInfo(ctx context.Context) {
	writeSection("PROJECT INFORMATION")

	crmService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create Cloud Resource Manager service: %v", err)
		return
	}

	project, err := crmService.Projects.Get(projectID).Do()
	if err != nil {
		log.Printf("Failed to get project info: %v", err)
		return
	}

	info := fmt.Sprintf("Name: %s\nProject ID: %s\nProject Number: %d\nState: %s\nCreate Time: %s",
		project.Name, project.ProjectId, project.ProjectNumber, project.LifecycleState, project.CreateTime)
	writeResource("Project", info)
}

func getStorageBuckets(ctx context.Context) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create storage client: %v", err)
		return
	}
	defer client.Close()

	it := client.Buckets(ctx, projectID)
	count := 0
	for {
		bucketAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("Failed to list buckets: %v", err)
			break
		}

		info := fmt.Sprintf("Name: %s\nLocation: %s\nStorage Class: %s\nCreated: %s",
			bucketAttrs.Name, bucketAttrs.Location, bucketAttrs.StorageClass, bucketAttrs.Created.Format(time.RFC3339))
		writeResource("Storage Bucket", info)
		count++
	}
	fmt.Printf("Found %d storage buckets\n", count)
}

func getIAMRoles(ctx context.Context) {
	crmService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create Cloud Resource Manager service: %v", err)
		return
	}

	policy, err := crmService.Projects.GetIamPolicy(projectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		log.Printf("Failed to get IAM policy: %v", err)
		return
	}

	for _, binding := range policy.Bindings {
		info := fmt.Sprintf("Role: %s\nMembers: %s", binding.Role, strings.Join(binding.Members, ", "))
		writeResource("IAM Binding", info)
	}
	fmt.Printf("Found %d IAM bindings\n", len(policy.Bindings))
}

func getServiceAccounts(ctx context.Context) {
	iamService, err := iam.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create IAM service: %v", err)
		return
	}

	parent := fmt.Sprintf("projects/%s", projectID)
	response, err := iamService.Projects.ServiceAccounts.List(parent).Do()
	if err != nil {
		log.Printf("Failed to list service accounts: %v", err)
		return
	}

	for _, sa := range response.Accounts {
		info := fmt.Sprintf("Email: %s\nDisplay Name: %s\nUnique ID: %s",
			sa.Email, sa.DisplayName, sa.UniqueId)
		writeResource("Service Account", info)
	}
	fmt.Printf("Found %d service accounts\n", len(response.Accounts))
}

func getComputeInstances(ctx context.Context, zone string) {
	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create compute service: %v", err)
		return
	}

	instances, err := computeService.Instances.List(projectID, zone+"-a").Do()
	if err != nil {
		// Silently skip if zone doesn't exist
		return
	}

	for _, instance := range instances.Items {
		info := fmt.Sprintf("Name: %s\nMachine Type: %s\nStatus: %s\nZone: %s\nCreated: %s",
			instance.Name, instance.MachineType, instance.Status,
			zone+"-a", instance.CreationTimestamp)

		if len(instance.NetworkInterfaces) > 0 && instance.NetworkInterfaces[0].AccessConfigs != nil &&
			len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
			info += fmt.Sprintf("\nExternal IP: %s", instance.NetworkInterfaces[0].AccessConfigs[0].NatIP)
		}

		writeResource("Compute Instance", info)
	}

	if len(instances.Items) > 0 {
		fmt.Printf("  Found %d compute instances in %s\n", len(instances.Items), zone)
	}
}

func getGKEClusters(ctx context.Context, location string) {
	client, err := container.NewClusterManagerClient(ctx)
	if err != nil {
		log.Printf("Failed to create GKE client: %v", err)
		return
	}
	defer client.Close()

	parent := fmt.Sprintf("projects/%s/locations/%s", projectID, location)
	response, err := client.ListClusters(ctx, &containerpb.ListClustersRequest{
		Parent: parent,
	})
	if err != nil {
		// Silently skip if location doesn't have GKE
		return
	}

	for _, cluster := range response.Clusters {
		info := fmt.Sprintf("Name: %s\nLocation: %s\nMaster Version: %s\nNode Count: %d\nStatus: %s",
			cluster.Name, cluster.Location, cluster.CurrentMasterVersion,
			cluster.CurrentNodeCount, cluster.Status)
		writeResource("GKE Cluster", info)
	}

	if len(response.Clusters) > 0 {
		fmt.Printf("  Found %d GKE clusters in %s\n", len(response.Clusters), location)
	}
}

func getCloudSQLInstances(ctx context.Context, region string) {
	sqlService, err := sqladmin.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create Cloud SQL service: %v", err)
		return
	}

	instances, err := sqlService.Instances.List(projectID).Do()
	if err != nil {
		log.Printf("Failed to list Cloud SQL instances: %v", err)
		return
	}

	count := 0
	for _, instance := range instances.Items {
		if strings.HasPrefix(instance.Region, region) {
			info := fmt.Sprintf("Name: %s\nDatabase Version: %s\nTier: %s\nRegion: %s\nState: %s",
				instance.Name, instance.DatabaseVersion, instance.Settings.Tier,
				instance.Region, instance.State)
			writeResource("Cloud SQL Instance", info)
			count++
		}
	}

	if count > 0 {
		fmt.Printf("  Found %d Cloud SQL instances in %s\n", count, region)
	}
}

func getVPCs(ctx context.Context, region string) {
	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create compute service: %v", err)
		return
	}

	networks, err := computeService.Networks.List(projectID).Do()
	if err != nil {
		log.Printf("Failed to list VPCs: %v", err)
		return
	}

	// VPCs are global, so we'll list them only once
	if region == regions[0] {
		for _, network := range networks.Items {
			info := fmt.Sprintf("Name: %s\nDescription: %s\nAuto Create Subnetworks: %v\nCreated: %s",
				network.Name, network.Description, network.AutoCreateSubnetworks, network.CreationTimestamp)
			writeResource("VPC Network", info)
		}
		fmt.Printf("  Found %d VPC networks\n", len(networks.Items))
	}
}

func getSubnets(ctx context.Context, region string) {
	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create compute service: %v", err)
		return
	}

	subnetworks, err := computeService.Subnetworks.List(projectID, region).Do()
	if err != nil {
		// Silently skip if region doesn't have subnets
		return
	}

	for _, subnet := range subnetworks.Items {
		info := fmt.Sprintf("Name: %s\nNetwork: %s\nIP Range: %s\nRegion: %s\nCreated: %s",
			subnet.Name, subnet.Network, subnet.IpCidrRange, subnet.Region, subnet.CreationTimestamp)
		writeResource("Subnet", info)
	}

	if len(subnetworks.Items) > 0 {
		fmt.Printf("  Found %d subnets in %s\n", len(subnetworks.Items), region)
	}
}

func getFirewallRules(ctx context.Context) {

	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create compute service: %v", err)
		return
	}

	firewalls, err := computeService.Firewalls.List(projectID).Do()
	if err != nil {
		log.Printf("Failed to list firewall rules: %v", err)
		return
	}

	for _, firewall := range firewalls.Items {
		info := fmt.Sprintf("Name: %s\nDirection: %s\nPriority: %d\nSource Ranges: %s\nTarget Tags: %s",
			firewall.Name, firewall.Direction, firewall.Priority,
			strings.Join(firewall.SourceRanges, ", "), strings.Join(firewall.TargetTags, ", "))
		writeResource("Firewall Rule", info)
	}
	fmt.Printf("Found %d firewall rules\n", len(firewalls.Items))
}

func getDisks(ctx context.Context, zone string) {
	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create compute service: %v", err)
		return
	}

	disks, err := computeService.Disks.List(projectID, zone+"-a").Do()
	if err != nil {
		// Silently skip if zone doesn't exist
		return
	}

	for _, disk := range disks.Items {
		info := fmt.Sprintf("Name: %s\nSize: %d GB\nType: %s\nStatus: %s\nZone: %s",
			disk.Name, disk.SizeGb, disk.Type, disk.Status, zone+"-a")
		writeResource("Persistent Disk", info)
	}

	if len(disks.Items) > 0 {
		fmt.Printf("  Found %d persistent disks in %s\n", len(disks.Items), zone)
	}
}

func getSnapshots(ctx context.Context) {

	computeService, err := compute.NewService(ctx)
	if err != nil {
		log.Printf("Failed to create compute service: %v", err)
		return
	}

	snapshots, err := computeService.Snapshots.List(projectID).Do()
	if err != nil {
		log.Printf("Failed to list snapshots: %v", err)
		return
	}

	for _, snapshot := range snapshots.Items {
		info := fmt.Sprintf("Name: %s\nDisk Size: %d GB\nStatus: %s\nCreated: %s",
			snapshot.Name, snapshot.DiskSizeGb, snapshot.Status, snapshot.CreationTimestamp)
		writeResource("Snapshot", info)
	}
	fmt.Printf("Found %d snapshots\n", len(snapshots.Items))
}
