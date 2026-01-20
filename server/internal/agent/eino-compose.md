# How Tools Work in ChatModelAgent

## Tool Execution Flow

In `ChatModelAgent`, tools are executed through a **ReAct (Reasoning and Acting) pattern**. When you configure a `ChatModelAgent` with tools, the agent creates an iterative loop that alternates between the chat model and tool execution. [1](#3-0) 

The execution follows this pattern:
1. **ChatModel generates** a response (potentially with tool calls)
2. **Branch node decides** whether to continue to tools or finish
3. **ToolsNode executes** the requested tools
4. Results are **fed back** to the ChatModel for the next iteration [2](#3-1) 

Tools are configured via the `ToolsConfig` which includes:
- A list of `BaseTool` implementations
- Parallel or sequential execution settings
- `ReturnDirectly` tools that cause immediate agent termination
- Tool call middlewares for customization [3](#3-2) 

## Pausing Execution for User Confirmation

To pause tool execution and wait for user confirmation, you implement a tool that uses the **interrupt/resume mechanism**. Here's how it works:

### 1. Tool Implementation Pattern

A tool that requires user approval follows this pattern: [4](#3-3) 

The key functions used are:

- **`compose.GetInterruptState[T](ctx)`** - Checks if the tool was previously interrupted and retrieves any saved state
- **`compose.StatefulInterrupt(ctx, info, state)`** - Pauses execution and saves internal state
- **`compose.GetResumeContext[T](ctx)`** - Checks if this is a resume operation and retrieves user-provided data [5](#3-4) [6](#3-5) [7](#3-6) 

### 2. Execution Flow

**First Invocation** (Initial Call):
1. Tool detects it's not in a resumed state (`wasInterrupted = false`)
2. Tool calls `compose.StatefulInterrupt()` to pause and save state
3. Execution stops and returns an interrupt event to the caller

**Resume Invocation** (After User Confirmation):
1. Tool detects it was interrupted (`wasInterrupted = true`)
2. Tool checks if it's the resume target using `GetResumeContext()`
3. If `isResumeTarget = true`, tool retrieves approval data and continues
4. If `isResumeTarget = false`, tool re-interrupts to preserve state (another component is being resumed)

### 3. Resuming with User Confirmation

To resume execution after receiving user confirmation, use the `Runner.ResumeWithParams()` method: [8](#3-7) 

**Example flow:** [9](#3-8) 

The interrupt event contains `InterruptContexts` with the interrupt ID. You extract the root cause interrupt ID and pass it along with the approval data to `ResumeWithParams()`. [10](#3-9) 

## Complete Example Pattern

Here's a simpler example without state saving: [11](#3-10) 

## Notes

- The interrupt mechanism works seamlessly with all ChatModelAgent configurations, including those with nested agents and complex tool hierarchies
- When a tool interrupts, the entire agent execution pauses and a checkpoint is saved (if a `CheckPointStore` is configured)
- Multiple tools can interrupt simultaneously - they will be merged into a single composite interrupt event with multiple `InterruptContexts`
- The `ResumeParams.Targets` map allows you to selectively resume specific interrupt points while others remain paused
- Tools marked as `ReturnDirectly` in the `ToolsConfig` will cause the agent to immediately return after execution, even if approved [12](#3-11)

### Citations

**File:** adk/chatmodel.go (L91-102)
```go
type ToolsConfig struct {
	compose.ToolsNodeConfig

	// ReturnDirectly specifies tools that cause the agent to return immediately when called.
	// If multiple listed tools are called simultaneously, only the first one triggers the return.
	// The map keys are tool names indicate whether the tool should trigger immediate return.
	ReturnDirectly map[string]bool

	// EmitInternalEvents indicates whether internal events from agentTool should be emitted
	// to the parent generator via a tool option injection at run-time.
	EmitInternalEvents bool
}
```

**File:** adk/chatmodel.go (L174-202)
```go
	ToolsConfig ToolsConfig

	// GenModelInput transforms instructions and input messages into the model's input format.
	// Optional. Defaults to defaultGenModelInput which combines instruction and messages.
	GenModelInput GenModelInput

	// Exit defines the tool used to terminate the agent process.
	// Optional. If nil, no Exit Action will be generated.
	// You can use the provided 'ExitTool' implementation directly.
	Exit tool.BaseTool

	// OutputKey stores the agent's response in the session.
	// Optional. When set, stores output via AddSessionValue(ctx, outputKey, msg.Content).
	OutputKey string

	// MaxIterations defines the upper limit of ChatModel generation cycles.
	// The agent will terminate with an error if this limit is exceeded.
	// Optional. Defaults to 20.
	MaxIterations int

	// Middlewares configures agent middleware for extending functionality.
	Middlewares []AgentMiddleware

	// ModelRetryConfig configures retry behavior for the ChatModel.
	// When set, the agent will automatically retry failed ChatModel calls
	// based on the configured policy.
	// Optional. If nil, no retry will be performed.
	ModelRetryConfig *ModelRetryConfig
}
```

**File:** adk/chatmodel.go (L815-831)
```go
		conf := &reactConfig{
			model:               a.model,
			toolsConfig:         &toolsNodeConf,
			toolsReturnDirectly: returnDirectly,
			agentName:           a.name,
			maxIterations:       a.maxIterations,
			beforeChatModel:     a.beforeChatModels,
			afterChatModel:      a.afterChatModels,
			modelRetryConfig:    a.modelRetryConfig,
		}

		g, err := newReact(ctx, conf)
		if err != nil {
			a.run = errFunc(err)
			return
		}

```

**File:** adk/prebuilt/supervisor/supervisor_test.go (L212-244)
```go
func (m *approvableTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, storedArguments := compose.GetInterruptState[string](ctx)
	if !wasInterrupted {
		return "", compose.StatefulInterrupt(ctx, &approvalInfo{
			ToolName:        m.name,
			ArgumentsInJSON: argumentsInJSON,
			ToolCallID:      compose.GetToolCallID(ctx),
		}, argumentsInJSON)
	}

	isResumeTarget, hasData, data := compose.GetResumeContext[*approvalResult](ctx)
	if !isResumeTarget {
		return "", compose.StatefulInterrupt(ctx, &approvalInfo{
			ToolName:        m.name,
			ArgumentsInJSON: storedArguments,
			ToolCallID:      compose.GetToolCallID(ctx),
		}, storedArguments)
	}

	if !hasData {
		return "", fmt.Errorf("tool '%s' resumed with no data", m.name)
	}

	if data.Approved {
		return fmt.Sprintf("Tool '%s' executed successfully with args: %s", m.name, storedArguments), nil
	}

	if data.DisapproveReason != nil {
		return fmt.Sprintf("Tool '%s' disapproved, reason: %s", m.name, *data.DisapproveReason), nil
	}

	return fmt.Sprintf("Tool '%s' disapproved", m.name), nil
}
```

**File:** adk/prebuilt/supervisor/supervisor_test.go (L441-457)
```go
	var toolInterruptID string
	for _, intCtx := range interruptEvent.Action.Interrupted.InterruptContexts {
		if intCtx.IsRootCause {
			toolInterruptID = intCtx.ID
			break
		}
	}
	assert.NotEmpty(t, toolInterruptID, "Should have a root cause interrupt ID")

	t.Logf("Resuming with approval for interrupt ID: %s", toolInterruptID)

	resumeIter, err := runner.ResumeWithParams(ctx, checkpointID, &adk.ResumeParams{
		Targets: map[string]any{
			toolInterruptID: &approvalResult{Approved: true},
		},
	})
	assert.NoError(t, err, "Resume should not error")
```

**File:** compose/resume.go (L25-34)
```go
// GetInterruptState provides a type-safe way to check for and retrieve the persisted state from a previous interruption.
// It is the primary function a component should use to understand its past state.
//
// It returns three values:
//   - wasInterrupted (bool): True if the node was part of a previous interruption, regardless of whether state was provided.
//   - state (T): The typed state object, if it was provided and matches type `T`.
//   - hasState (bool): True if state was provided during the original interrupt and successfully cast to type `T`.
func GetInterruptState[T any](ctx context.Context) (wasInterrupted bool, hasState bool, state T) {
	return core.GetInterruptState[T](ctx)
}
```

**File:** compose/resume.go (L36-77)
```go
// GetResumeContext checks if the current component is the target of a resume operation
// and retrieves any data provided by the user for that resumption.
//
// This function is typically called *after* a component has already determined it is in a
// resumed state by calling GetInterruptState.
//
// It returns three values:
//   - isResumeFlow: A boolean that is true if the current component's address was explicitly targeted
//     by a call to Resume() or ResumeWithData().
//   - hasData: A boolean that is true if data was provided for this component (i.e., not nil).
//   - data: The typed data provided by the user.
//
// ### How to Use This Function: A Decision Framework
//
// The correct usage pattern depends on the application's desired resume strategy.
//
// #### Strategy 1: Implicit "Resume All"
// In some use cases, any resume operation implies that *all* interrupted points should proceed.
// For example, if an application's UI only provides a single "Continue" button for a set of
// interruptions. In this model, a component can often just use `GetInterruptState` to see if
// `wasInterrupted` is true and then proceed with its logic, as it can assume it is an intended target.
// It may still call `GetResumeContext` to check for optional data, but the `isResumeFlow` flag is less critical.
//
// #### Strategy 2: Explicit "Targeted Resume" (Most Common)
// For applications with multiple, distinct interrupt points that must be resumed independently, it is
// crucial to differentiate which point is being resumed. This is the primary use case for the `isResumeFlow` flag.
//   - If `isResumeFlow` is `true`: Your component is the explicit target. You should consume
//     the `data` (if any) and complete your work.
//   - If `isResumeFlow` is `false`: Another component is the target. You MUST re-interrupt
//     (e.g., by returning `StatefulInterrupt(...)`) to preserve your state and allow the
//     resume signal to propagate.
//
// ### Guidance for Composite Components
//
// Composite components (like `Graph` or other `Runnable`s that contain sub-processes) have a dual role:
//  1. Check for Self-Targeting: A composite component can itself be the target of a resume
//     operation, for instance, to modify its internal state. It may call `GetResumeContext`
//     to check for data targeted at its own address.
//  2. Act as a Conduit: After checking for itself, its primary role is to re-execute its children,
//     allowing the resume context to flow down to them. It must not consume a resume signal
//     intended for one of its descendants.
func GetResumeContext[T any](ctx context.Context) (isResumeFlow bool, hasData bool, data T) {
```

**File:** compose/resume.go (L120-137)
```go
	return core.BatchResumeWithData(ctx, resumeData)
}

func getNodePath(ctx context.Context) (*NodePath, bool) {
	currentAddress := GetCurrentAddress(ctx)
	if len(currentAddress) == 0 {
		return nil, false
	}

	nodePath := make([]string, 0, len(currentAddress))
	for _, p := range currentAddress {
		if p.Type == AddressSegmentRunnable {
			nodePath = []string{}
			continue
		}

		nodePath = append(nodePath, p.ID)
	}
```

**File:** adk/runner.go (L50-58)
```go
// ResumeParams contains all parameters needed to resume an execution.
// This struct provides an extensible way to pass resume parameters without
// requiring breaking changes to method signatures.
type ResumeParams struct {
	// Targets contains the addresses of components to be resumed as keys,
	// with their corresponding resume data as values
	Targets map[string]any
	// Future extensible fields can be added here without breaking changes
}
```

**File:** adk/runner.go (L119-139)
```go
// ResumeWithParams continues an interrupted execution from a checkpoint with specific parameters.
// This is the most common and powerful way to resume, allowing you to target specific interrupt points
// (identified by their address/ID) and provide them with data.
//
// The params.Targets map should contain the addresses of the components to be resumed as keys. These addresses
// can point to any interruptible component in the entire execution graph, including ADK agents, compose
// graph nodes, or tools. The value can be the resume data for that component, or `nil` if no data is needed.
//
// When using this method:
//   - Components whose addresses are in the params.Targets map will receive `isResumeFlow = true` when they
//     call `GetResumeContext`.
//   - Interrupted components whose addresses are NOT in the params.Targets map must decide how to proceed:
//     -- "Leaf" components (the actual root causes of the original interrupt) MUST re-interrupt themselves
//     to preserve their state.
//     -- "Composite" agents (like SequentialAgent or ChatModelAgent) should generally proceed with their
//     execution. They act as conduits, allowing the resume signal to flow to their children. They will
//     naturally re-interrupt if one of their interrupted children re-interrupts, as they receive the
//     new `CompositeInterrupt` signal from them.
func (r *Runner) ResumeWithParams(ctx context.Context, checkPointID string, params *ResumeParams, opts ...AgentRunOption) (*AsyncIterator[*AgentEvent], error) {
	return r.resume(ctx, checkPointID, params.Targets, opts...)
}
```

**File:** adk/prebuilt/planexecute/plan_execute_test.go (L747-760)
```go
func (m *interruptibleTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, _ := compose.GetInterruptState[any](ctx)
	if !wasInterrupted {
		return "", compose.Interrupt(ctx, fmt.Sprintf("Tool '%s' requires human approval", m.name))
	}

	isResumeTarget, hasData, data := compose.GetResumeContext[string](ctx)
	if !isResumeTarget {
		return "", compose.Interrupt(ctx, fmt.Sprintf("Tool '%s' requires human approval", m.name))
	}

	if hasData {
		return fmt.Sprintf("Approved action executed with data: %s", data), nil
	}
```
