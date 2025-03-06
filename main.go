package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// ConfigLoader defines an interface for loading AWS configuration.
// This interface creation is necessary for mocking.
type ConfigLoader interface {
	LoadDefaultConfigMethod(ctx context.Context) (aws.Config, error)
}

// DefaultConfigLoader is the type upon which we call the LoadDefaultConfigMethod Method.
// This type creation is necessary for mocking.
type DefaultConfigLoader struct{}

// LoadDefaultConfigMethod implements the ConfigLoader interface using the AWS SDK.
// LoadDefaultConfigMethod is the func converted to method necessary for mocking.
func (l *DefaultConfigLoader) LoadDefaultConfigMethod(ctx context.Context) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx)
}

// This is the STSClient interface for STS operations.
type STSClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// EC2Client interface for EC2 operations
type EC2Client interface {
	DescribeRegions(ctx context.Context, params *ec2.DescribeRegionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRegionsOutput, error)
}

// EKSClient interface for EKS operations
type EKSClient interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

// Clusters holds information about EKS clusters
type Clusters struct {
	Names []string
	Urls  []string
}

func main() {
	ctx := context.Background()
	dcl := DefaultConfigLoader{}

	// Create clients
	stsClient := newSTSClient(ctx, &dcl)
	ec2Client := newEC2Client(ctx, &dcl)

	// Get account info
	account, err := getAccountInfo(ctx, stsClient)
	if err != nil {
		log.Fatalf("Failed to get account info: %v", err)
	}
	fmt.Printf("Analyzing EKS clusters for AWS Account: %s\n\n", *account)

	// Get regions
	regions, err := listAwsRegions(ec2Client)
	if err != nil {
		log.Fatalf("Failed to describe regions: %v", err)
	}

	// Print regions
	printRegions(regions)

	// Get EKS clusters across all regions
	eksClient := newEKSClient(ctx, &dcl)
	clusters, err := getAllClusters(ctx, regions, eksClient)
	if err != nil {
		log.Fatalf("Error getting clusters: %v", err)
	}

	fmt.Printf("Total clusters found: %d\n", len(clusters.Names))

	// Get cluster endpoints
	err = getClusterEndpoints(ctx, eksClient, clusters)
	if err != nil {
		log.Fatalf("Error getting cluster endpoints: %v", err)
	}

	// Print endpoints
	for _, v := range clusters.Urls {
		fmt.Println(v)
	}
}

// getAccountInfo retrieves the AWS account ID
func getAccountInfo(ctx context.Context, client STSClient) (*string, error) {
	clientDetails, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}
	return clientDetails.Account, nil
}

// printRegions prints the list of AWS regions
func printRegions(regions []string) {
	fmt.Println("Available AWS Regions:")
	for _, region := range regions {
		fmt.Printf("* %s \n", region)
	}
}

// getAllClusters gets all EKS clusters across specified regions
func getAllClusters(ctx context.Context, regions []string, baseClient EKSClient) (*Clusters, error) {
	clusters := &Clusters{}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Iterate through each region
	for _, region := range regions {
		// Create a new EKS client for each region
		regionCfg := cfg.Copy()
		regionCfg.Region = region

		eksClient := eks.NewFromConfig(regionCfg)

		fmt.Printf("Checking region: %s\n", region)

		// List clusters in this region
		clustersListOutput, err := eksClient.ListClusters(ctx, &eks.ListClustersInput{})
		if err != nil {
			fmt.Printf("Error listing clusters in region %s: %v\n", region, err)
			continue // Skip to next region instead of fatal error
		}

		for _, v := range clustersListOutput.Clusters {
			clusters.Names = append(clusters.Names, v)
			fmt.Printf("Found cluster: %s in region: %s\n", v, region)
		}
	}

	return clusters, nil
}

// getClusterEndpoints retrieves the endpoints for each cluster
func getClusterEndpoints(ctx context.Context, client EKSClient, clusters *Clusters) error {
	for _, v := range clusters.Names {
		clusterInfo, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &v})
		if err != nil {
			return err
		}
		clusters.Urls = append(clusters.Urls, *clusterInfo.Cluster.Endpoint)
	}
	return nil
}

// Create a new STS client using the provided config loader
func newSTSClient(ctx context.Context, loader ConfigLoader) *sts.Client {
	cfg, err := loader.LoadDefaultConfigMethod(ctx)
	if err != nil {
		log.Fatal(err)
	}
	return sts.NewFromConfig(cfg)
}

// Create a new EKS client using the provided config loader
func newEKSClient(ctx context.Context, loader ConfigLoader) *eks.Client {
	cfg, err := loader.LoadDefaultConfigMethod(ctx)
	if err != nil {
		log.Fatal(err)
	}
	return eks.NewFromConfig(cfg)
}

// Create a new EC2 client using the provided config loader
func newEC2Client(ctx context.Context, loader ConfigLoader) *ec2.Client {
	cfg, err := loader.LoadDefaultConfigMethod(ctx)
	if err != nil {
		log.Fatal(err)
	}
	return ec2.NewFromConfig(cfg)
}

// listAwsRegions gets all available AWS regions
func listAwsRegions(ec2Client EC2Client) ([]string, error) {
	var regionsSlice []string
	regionsOutput, err := ec2Client.DescribeRegions(context.TODO(), &ec2.DescribeRegionsInput{
		// Optional: Set to true to include disabled regions
		AllRegions: aws.Bool(true),
	})

	if err != nil {
		fmt.Println(err)
		return []string{}, err
	}

	for _, region := range regionsOutput.Regions {
		regionsSlice = append(regionsSlice, string(*region.RegionName))
	}

	return regionsSlice, err
}
