package advisor

// Severity indicates how impactful a recommendation is.
type Severity string

const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
)

// Category classifies what kind of optimisation is being suggested.
type Category string

const (
	CategoryRightSizing  Category = "RIGHT_SIZING"
	CategoryIdleWorkload Category = "IDLE_WORKLOAD"
	CategoryNodegroup    Category = "NODEGROUP"
	CategoryInstanceType Category = "INSTANCE_TYPE"
	CategorySpot         Category = "SPOT_OPPORTUNITY"
)

// ResourceRef identifies the Kubernetes or cloud resource the recommendation targets.
type ResourceRef struct {
	Kind      string // Deployment, Node, Nodegroup
	Namespace string // empty for cluster-scoped resources
	Name      string
}

// Recommendation is a single actionable cost-optimisation suggestion.
type Recommendation struct {
	ID                     string            `json:"id" yaml:"id"`
	Category               Category          `json:"category" yaml:"category"`
	Severity               Severity          `json:"severity" yaml:"severity"`
	Resource               ResourceRef       `json:"resource" yaml:"resource"`
	Summary                string            `json:"summary" yaml:"summary"`
	Detail                 string            `json:"detail" yaml:"detail"`
	EstimatedMonthlySaving float64           `json:"estimatedMonthlySavingUSD" yaml:"estimatedMonthlySavingUSD"`
	CurrentState           map[string]string `json:"currentState" yaml:"currentState"`
	SuggestedAction        map[string]string `json:"suggestedAction" yaml:"suggestedAction"`
	CheckName              string            `json:"checkName" yaml:"checkName"`
}
