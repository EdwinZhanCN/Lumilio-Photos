// Package pins promotes session refs to durable widgets. A pin stores the
// frozen snapshot and the plan that produced it; frozen pins always serve
// the stored snapshot, live pins replay the plan on hydration when the plan
// is a self-contained producer expression (filter_assets / search_*).
// Transformed or combined refs pin as frozen — their plans reference session
// refs that do not outlive the conversation.
//
//atlas:group agent
package pins
