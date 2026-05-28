package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingtypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

// PricingClient fetches on-demand EC2 pricing.
// The AWS Pricing API is only available in us-east-1 regardless of cluster region.
type PricingClient struct {
	svc *pricing.Client
}

// NewPricingClient creates a Pricing API client always pointed at us-east-1.
func NewPricingClient(ctx context.Context, profile, roleARN string) (*PricingClient, error) {
	// Pricing API endpoint is only available in us-east-1.
	cfg, err := NewConfig(ctx, "us-east-1", profile, roleARN)
	if err != nil {
		return nil, err
	}
	return &PricingClient{svc: pricing.NewFromConfig(cfg)}, nil
}

// NewPricingClientFromConfig creates a Pricing API client by overriding the
// region of an existing AWS config to us-east-1.
func NewPricingClientFromConfig(cfg awssdk.Config) (*PricingClient, error) {
	// Build a new config for us-east-1 using the same credential chain.
	ctx := context.Background()
	pricingCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(cfg.Credentials),
	)
	if err != nil {
		return nil, fmt.Errorf("build pricing config: %w", err)
	}
	return &PricingClient{svc: pricing.NewFromConfig(pricingCfg)}, nil
}

// FetchOnDemandPrices returns the on-demand Linux price per hour for each
// requested instance type, keyed by instance type name.
// Types not found in the Pricing API are omitted from the result.
func (c *PricingClient) FetchOnDemandPrices(ctx context.Context, instanceTypes []string) (map[string]float64, error) {
	if len(instanceTypes) == 0 {
		return nil, nil
	}
	slog.Debug("fetching on-demand pricing", "instance_types", instanceTypes)

	result := make(map[string]float64, len(instanceTypes))
	for _, it := range instanceTypes {
		price, err := c.fetchPrice(ctx, it)
		if err != nil {
			// Non-fatal: some instance types may not have pricing data.
			slog.Debug("pricing lookup failed", "instance_type", it, "err", err)
			continue
		}
		slog.Debug("pricing fetched", "instance_type", it, "price_per_hr", price)
		result[it] = price
	}
	slog.Debug("pricing fetch complete", "found", len(result), "requested", len(instanceTypes))
	return result, nil
}

func (c *PricingClient) fetchPrice(ctx context.Context, instanceType string) (float64, error) {
	out, err := c.svc.GetProducts(ctx, &pricing.GetProductsInput{
		ServiceCode: awssdk.String("AmazonEC2"),
		Filters: []pricingtypes.Filter{
			{Type: pricingtypes.FilterTypeTermMatch, Field: awssdk.String("instanceType"), Value: awssdk.String(instanceType)},
			{Type: pricingtypes.FilterTypeTermMatch, Field: awssdk.String("operatingSystem"), Value: awssdk.String("Linux")},
			{Type: pricingtypes.FilterTypeTermMatch, Field: awssdk.String("tenancy"), Value: awssdk.String("Shared")},
			{Type: pricingtypes.FilterTypeTermMatch, Field: awssdk.String("preInstalledSw"), Value: awssdk.String("NA")},
			{Type: pricingtypes.FilterTypeTermMatch, Field: awssdk.String("capacitystatus"), Value: awssdk.String("Used")},
		},
		MaxResults: awssdk.Int32(1),
	})
	if err != nil {
		return 0, fmt.Errorf("get products for %s: %w", instanceType, err)
	}
	if len(out.PriceList) == 0 {
		return 0, fmt.Errorf("no pricing found for %s", instanceType)
	}

	return parsePriceFromJSON(out.PriceList[0])
}

// parsePriceFromJSON extracts the USD on-demand price from an AWS Pricing API
// product JSON blob.
func parsePriceFromJSON(raw string) (float64, error) {
	var doc struct {
		Terms struct {
			OnDemand map[string]struct {
				PriceDimensions map[string]struct {
					PricePerUnit struct {
						USD string `json:"USD"`
					} `json:"pricePerUnit"`
				} `json:"priceDimensions"`
			} `json:"OnDemand"`
		} `json:"terms"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return 0, fmt.Errorf("parse pricing JSON: %w", err)
	}
	for _, term := range doc.Terms.OnDemand {
		for _, dim := range term.PriceDimensions {
			if dim.PricePerUnit.USD != "" {
				return strconv.ParseFloat(dim.PricePerUnit.USD, 64)
			}
		}
	}
	return 0, fmt.Errorf("no USD price found in pricing response")
}
