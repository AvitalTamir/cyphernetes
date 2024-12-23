package provider

// RelationshipType represents the type of relationship between resources
type RelationshipType string

// ComparisonType represents how fields should be compared
type ComparisonType string

const (
	// Comparison types
	ExactMatch ComparisonType = "EXACT_MATCH"
	RegexMatch ComparisonType = "REGEX_MATCH"
	// Add other comparison types as needed
)

// MatchCriterion defines how to match fields between resources
type MatchCriterion struct {
	FieldA         string         // JSONPath expression for field in resource A
	FieldB         string         // JSONPath expression for field in resource B
	ComparisonType ComparisonType // How to compare the fields
}

// RelationshipRule defines a relationship between two Kubernetes resource kinds
type RelationshipRule struct {
	KindA         string           // The first resource kind
	KindB         string           // The second resource kind
	Relationship  RelationshipType // The type of relationship
	MatchCriteria []MatchCriterion // Criteria for matching resources
}
