package model

// Space is a Notion workspace row from the `space` table.
type Space struct {
	ID               string
	Name             string
	PlanType         string
	SubscriptionTier string
}
