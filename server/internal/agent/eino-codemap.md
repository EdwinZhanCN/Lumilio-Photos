## Single Agent System with ChatModelAgent
Complete flow of building and running a single agent system with ChatModelAgent, including configuration, execution, interrupt handling, resume functionality, and session state management. Key locations: agent creation [1a], runner initialization [2a], interrupt handling [4a], checkpoint saving [5a], and session state operations [6a].
### 1. Creating and Configuring ChatModelAgent
Setting up a ChatModelAgent with model, tools, and configuration options
### 1a. Agent Constructor (`chatmodel.go:237`)
Creates a new ChatModelAgent with provided configuration
```text
func NewChatModelAgent(_ context.Context, config *ChatModelAgentConfig) (*ChatModelAgent, error)
```
### 1b. Configuration Structure (`chatmodel.go:159`)
Defines all configuration options including name, model, tools, and output key
```text
type ChatModelAgentConfig struct
```
### 1c. Session Output Storage (`chatmodel.go:186`)
Key for storing agent output in session state
```text
OutputKey string
```
### 1d. Execution Limits (`chatmodel.go:192`)
Maximum number of ChatModel generation cycles
```text
MaxIterations int
```
### 2. Runner Initialization and Execution
Creating a Runner to manage agent lifecycle with checkpoint support
### 2a. Runner Constructor (`runner.go:62`)
Creates a new Runner with agent, streaming, and checkpoint configuration
```text
func NewRunner(_ context.Context, conf RunnerConfig) *Runner
```
### 2b. Checkpoint Store (`runner.go:40`)
Storage for persisting agent state during interrupts
```text
CheckPointStore compose.CheckPointStore
```
### 2c. Execute Agent (`runner.go:74`)
Starts new agent execution with messages and returns event iterator
```text
func (r *Runner) Run(ctx context.Context, messages []Message, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]
```
### 2d. Convenience Query (`runner.go:101`)
Simplified method for single query execution
```text
func (r *Runner) Query(ctx context.Context, query string, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]
```
### 3. Session State Management
Managing shared state across agent execution
### 3a. Set Session Value (`runctx.go:102`)
Stores a key-value pair in the shared session state
```text
func AddSessionValue(ctx context.Context, key string, value any)
```
### 3b. Get Session Value (`runctx.go:122`)
Retrieves a value from the session state
```text
func GetSessionValue(ctx context.Context, key string) (any, bool)
```
### 3c. Set Multiple Values (`runctx.go:112`)
Stores multiple key-value pairs at once
```text
func AddSessionValues(ctx context.Context, kvs map[string]any)
```
### 3d. Initialize Session (`runner.go:87`)
Adds initial session values before execution
```text
AddSessionValues(ctx, o.sessionValues)
```
### 4. Interrupt and Resume System
Handling agent interrupts and resuming from checkpoints
### 4a. Basic Interrupt (`interrupt.go:60`)
Creates an interrupt action without state preservation
```text
func Interrupt(ctx context.Context, info any) *AgentEvent
```
### 4b. Stateful Interrupt (`interrupt.go:79`)
Creates an interrupt with internal state serialization
```text
func StatefulInterrupt(ctx context.Context, info any, state any) *AgentEvent
```
### 4c. Targeted Resume (`runner.go:137`)
Resumes execution with specific parameters for interrupt points
```text
func (r *Runner) ResumeWithParams(ctx context.Context, checkPointID string, params *ResumeParams, opts ...AgentRunOption) (*AsyncIterator[*AgentEvent], error)
```
### 4d. Simple Resume (`runner.go:114`)
Resumes all interrupted points without specific data
```text
func (r *Runner) Resume(ctx context.Context, checkPointID string, opts ...AgentRunOption) (*AsyncIterator[*AgentEvent], error)
```
### 5. Checkpoint Persistence
Saving and loading agent state during interrupts
### 5a. Save Checkpoint (`interrupt.go:201`)
Serializes and stores agent state to checkpoint store
```text
func (r *Runner) saveCheckPoint(ctx context.Context, key string, info *InterruptInfo, is *core.InterruptSignal) error
```
### 5b. Load Checkpoint (`interrupt.go:178`)
Deserializes and restores agent state from checkpoint store
```text
func (r *Runner) loadCheckPoint(ctx context.Context, checkpointID string) (context.Context, *runContext, *ResumeInfo, error)
```
### 5c. Auto-Save on Interrupt (`runner.go:235`)
Automatically saves checkpoint when interrupt occurs
```text
err := r.saveCheckPoint(ctx, *checkPointID, &InterruptInfo{Data: legacyData}, interruptSignal)
```
### 5d. Checkpoint Schema (`interrupt.go:169`)
Structure for persisted checkpoint data
```text
type serialization struct
```
### 6. Agent Execution Flow
Complete flow from agent run to event generation
### 6a. Agent Run Method (`chatmodel.go:894`)
Main entry point for agent execution
```text
func (a *ChatModelAgent) Run(ctx context.Context, input *AgentInput, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]
```
### 6b. Execute Run Function (`chatmodel.go:912`)
Calls the built run function with bridge store for checkpoints
```text
run(ctx, input, generator, newBridgeStore(), co...)
```
### 6c. Store Output (`chatmodel.go:677`)
Automatically stores agent output in session if OutputKey is set
```text
func setOutputToSession(ctx context.Context, msg Message, msgStream MessageStream, outputKey string) error
```
### 6d. Agent Resume Method (`chatmodel.go:918`)
Resumes agent execution from saved checkpoint
```text
func (a *ChatModelAgent) Resume(ctx context.Context, info *ResumeInfo, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]
```
### 7. Example Integration Pattern
Complete test example showing interrupt/resume flow
### 7a. Setup Runner with Store (`interrupt_test.go:110`)
Creates runner with checkpoint store for interrupt support
```text
runner := NewRunner(ctx, RunnerConfig{Agent: agent, EnableStreaming: true, CheckPointStore: store})
```
### 7b. Trigger Interrupt (`interrupt_test.go:90`)
Agent creates interrupt event during execution
```text
intEvent := Interrupt(ctx, data)
```
### 7c. Resume with Target (`interrupt_test.go:129`)
Resumes execution targeting specific interrupt context ID
```text
iter, err := runner.ResumeWithParams(ctx, "1", &ResumeParams{Targets: map[string]any{interruptEvent.Action.Interrupted.InterruptContexts[0].ID: nil}})
```
