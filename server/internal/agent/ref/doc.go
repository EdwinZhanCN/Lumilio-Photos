// Package ref implements the server-side handle store for the agent ref
// system. Agent tools exchange ordered asset-ID snapshots through short
// ref ids instead of inlining asset data into the LLM context; the frontend
// hydrates refs over HTTP, so asset data never crosses the model boundary.
//
//atlas:group agent
package ref
