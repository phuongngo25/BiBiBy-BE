package domain

// Reason structurally describes why a specific restriction rule was triggered.
type Reason struct {
	RuleID      uint     `json:"rule_id"`
	ConditionID string   `json:"condition_id"`
	Type        RuleType `json:"type"`
	Target      string   `json:"target"`
	Message     string   `json:"message"`
}

// Explanation represents the final output of the In-Memory Evaluation Engine.
type Explanation struct {
	// IsSafe is false ONLY if a Hard constraint is violated (e.g., deadly allergy).
	IsSafe        bool `json:"is_safe"`
	
	// IsRecommended is false if ANY constraint (Hard or Soft) is violated.
	IsRecommended bool `json:"is_recommended"`
	
	// Warnings contains all structured reasons for the violations (both hard and soft).
	Warnings      []Reason `json:"warnings"`
}

// AddWarning appends a Reason to the Explanation and flags the boolean triggers accordingly.
func (e *Explanation) AddWarning(reason Reason, isHard bool) {
	e.Warnings = append(e.Warnings, reason)
	
	// Any warning means it is no longer purely recommended
	e.IsRecommended = false
	
	// Only Hard warnings render the food fundamentally unsafe
	if isHard {
		e.IsSafe = false
	}
}

// NewExplanation initializes an optimistic (Safe & Recommended) Explanation.
func NewExplanation() Explanation {
	return Explanation{
		IsSafe:        true,
		IsRecommended: true,
		Warnings:      make([]Reason, 0),
	}
}
