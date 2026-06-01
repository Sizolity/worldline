// Package view renders bounded projections of world state for callers.
//
// Views must not mutate world state. Character-facing views must be owner-aware
// and must not expose hidden narrator or world knowledge.
package view
