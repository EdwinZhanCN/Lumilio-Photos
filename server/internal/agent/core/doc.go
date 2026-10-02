// Package core runs the Lumilio Agent.
//
// [AgentService] owns conversation threads and runs: [AgentService.AskAgent]
// starts a model run in a mode (organize, analyze, review, curate) whose tool
// set is fixed by the [ToolRegistry]; [AgentService.ResumeAgent] continues
// after an interrupt; runs are cancelled through the process-local
// [RunRegistry]. Progress, tool execution, and widget data reach the browser
// as [SideChannelEvent] envelopes.
//
// Tools never return asset data to the model; they produce references (see
// [server/internal/agent/ref]). Mutating tools are effects: the
// [EffectRuntime] prepares a pending effect, the user confirms or rejects it,
// and [EffectRuntime.Commit] applies it with its receipt in one catalog
// transaction.
//
//atlas:group agent
package core
