# Implementing a Single Agent System with ChatModelAgent and Runner

Here's a comprehensive guide on implementing a single agent system using ChatModelAgent with Runner, including interrupt/resume functionality and session state management.

## 1. Basic Setup: Creating a ChatModelAgent

To create a ChatModelAgent, you need to configure it with a `ChatModelAgentConfig`: [1](#1-0) 

The configuration requires:
- **Name**: Unique agent identifier
- **Description**: Capability description for agent discovery
- **Model**: A `ToolCallingChatModel` implementation
- **Instruction**: System prompt (supports session value placeholders like `{Time}`)
- **ToolsConfig**: Optional tool configuration
- **MaxIterations**: Default is 20 if not specified

## 2. Using Runner to Execute the Agent

The `Runner` is the primary entry point for executing agents with lifecycle management: [2](#1-1) 

Basic execution pattern: [3](#1-2) 

The `Runner.Query()` method is a convenience wrapper: [4](#1-3) 

## 3. Session State Management

Session state allows you to share key-value pairs across the agent execution:

### Setting Session Values [5](#1-4) [6](#1-5) 

### Getting Session Values [7](#1-6) [8](#1-7) 

### Automatic Output Storage

You can configure the agent to automatically store its output in the session: [9](#1-8) 

The implementation stores output automatically: [10](#1-9) 

## 4. Interrupt and Resume Functionality

### Creating Interrupts

Agents can interrupt their execution using two methods:

**Basic Interrupt** (no internal state): [11](#1-10) 

**Stateful Interrupt** (with internal state): [12](#1-11) 

### Understanding ResumeInfo

When resuming, agents receive a `ResumeInfo` structure: [13](#1-12) 

### Resume Methods

The Runner provides two resume methods:

**Simple Resume** (implicit resume all): [14](#1-13) 

**Resume with Params** (targeted resume with data): [15](#1-14) 

The `ResumeParams` structure: [16](#1-15) 

### ChatModelAgent Resume Implementation

ChatModelAgent's Resume method handles checkpoint restoration: [17](#1-16) 

### Modifying History on Resume

You can modify the agent's message history during resume: [18](#1-17) 

## 5. Checkpoint Persistence

The Runner automatically handles checkpoint persistence when configured with a `CheckPointStore`: [19](#1-18) 

Checkpoints are saved when an interrupt occurs: [20](#1-19) 

And loaded during resume: [21](#1-20) 

## 6. Complete Integration Pattern Example

Here's the typical flow from the test files: [22](#1-21) 

This test demonstrates:
1. Creating an agent that interrupts during execution
2. Using Runner with CheckPointStore
3. Receiving interrupt events with context information
4. Resuming with targeted parameters using interrupt context IDs

## 7. Advanced: ChatModelAgent with Tools

When using tools, ChatModelAgent automatically builds a ReAct graph: [23](#1-22) 

## Notes

**Key Integration Points:**
- The Runner wraps the agent's iterator to handle checkpoint persistence transparently
- Session state is thread-safe and shared across the execution
- Interrupt contexts provide hierarchical information about where the interrupt occurred
- The `InterruptCtx.ID` is used to target specific interrupts when resuming
- ChatModelAgent uses a "bridge store" internally for checkpoint serialization
- Session values support template substitution in agent instructions

**Best Practices:**
- Always use `WithCheckPointID()` option when you need interrupt/resume capability
- Use `ResumeWithParams` for fine-grained control over which interrupts to resume
- Store intermediate results in session values for cross-agent communication
- The `EnableStreaming` flag is automatically preserved through interrupts and resumes

### Citations

**File:** adk/chatmodel.go (L185-187)
```go
	// OutputKey stores the agent's response in the session.
	// Optional. When set, stores output via AddSessionValue(ctx, outputKey, msg.Content).
	OutputKey string
```

**File:** adk/chatmodel.go (L237-287)
```go
func NewChatModelAgent(_ context.Context, config *ChatModelAgentConfig) (*ChatModelAgent, error) {
	if config.Name == "" {
		return nil, errors.New("agent 'Name' is required")
	}
	if config.Description == "" {
		return nil, errors.New("agent 'Description' is required")
	}
	if config.Model == nil {
		return nil, errors.New("agent 'Model' is required")
	}

	genInput := defaultGenModelInput
	if config.GenModelInput != nil {
		genInput = config.GenModelInput
	}

	beforeChatModels := make([]func(context.Context, *ChatModelAgentState) error, 0)
	afterChatModels := make([]func(context.Context, *ChatModelAgentState) error, 0)
	sb := &strings.Builder{}
	sb.WriteString(config.Instruction)
	tc := config.ToolsConfig
	for _, m := range config.Middlewares {
		sb.WriteString("\n")
		sb.WriteString(m.AdditionalInstruction)
		tc.Tools = append(tc.Tools, m.AdditionalTools...)

		if m.WrapToolCall.Invokable != nil || m.WrapToolCall.Streamable != nil {
			tc.ToolCallMiddlewares = append(tc.ToolCallMiddlewares, m.WrapToolCall)
		}
		if m.BeforeChatModel != nil {
			beforeChatModels = append(beforeChatModels, m.BeforeChatModel)
		}
		if m.AfterChatModel != nil {
			afterChatModels = append(afterChatModels, m.AfterChatModel)
		}
	}

	return &ChatModelAgent{
		name:             config.Name,
		description:      config.Description,
		instruction:      sb.String(),
		model:            config.Model,
		toolsConfig:      tc,
		genModelInput:    genInput,
		exit:             config.Exit,
		outputKey:        config.OutputKey,
		maxIterations:    config.MaxIterations,
		beforeChatModels: beforeChatModels,
		afterChatModels:  afterChatModels,
		modelRetryConfig: config.ModelRetryConfig,
	}, nil
```

**File:** adk/chatmodel.go (L677-690)
```go
func setOutputToSession(ctx context.Context, msg Message, msgStream MessageStream, outputKey string) error {
	if msg != nil {
		AddSessionValue(ctx, outputKey, msg.Content)
		return nil
	}

	concatenated, err := schema.ConcatMessageStream(msgStream)
	if err != nil {
		return err
	}

	AddSessionValue(ctx, outputKey, concatenated.Content)
	return nil
}
```

**File:** adk/chatmodel.go (L698-704)
```go
// ChatModelAgentResumeData holds data that can be provided to a ChatModelAgent during a resume operation
// to modify its behavior. It is provided via the adk.ResumeWithData function.
type ChatModelAgentResumeData struct {
	// HistoryModifier is a function that can transform the agent's message history before it is sent to the model.
	// This allows for adding new information or context upon resumption.
	HistoryModifier func(ctx context.Context, history []Message) []Message
}
```

**File:** adk/chatmodel.go (L814-886)
```go
		// react
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

		a.run = func(ctx context.Context, input *AgentInput, generator *AsyncGenerator[*AgentEvent], store *bridgeStore,
			opts ...compose.Option) {
			var compileOptions []compose.GraphCompileOption
			compileOptions = append(compileOptions,
				compose.WithGraphName(a.name),
				compose.WithCheckPointStore(store),
				compose.WithSerializer(&gobSerializer{}),
				// ensure the graph won't exceed max steps due to max iterations
				compose.WithMaxRunSteps(math.MaxInt))

			runnable, err_ := compose.NewChain[*AgentInput, Message]().
				AppendLambda(
					compose.InvokableLambda(func(ctx context.Context, input *AgentInput) ([]Message, error) {
						return a.genModelInput(ctx, instruction, input)
					}),
				).
				AppendGraph(g, compose.WithNodeName("ReAct"), compose.WithGraphCompileOptions(compose.WithMaxRunSteps(math.MaxInt))).
				Compile(ctx, compileOptions...)
			if err_ != nil {
				generator.Send(&AgentEvent{Err: err_})
				return
			}

			callOpt := genReactCallbacks(ctx, a.name, generator, input.EnableStreaming, store, a.modelRetryConfig)
			var runOpts []compose.Option
			runOpts = append(runOpts, opts...)
			runOpts = append(runOpts, callOpt)
			if a.toolsConfig.EmitInternalEvents {
				runOpts = append(runOpts, compose.WithToolsNodeOption(compose.WithToolOption(withAgentToolEventGenerator(generator))))
			}
			if input.EnableStreaming {
				runOpts = append(runOpts, compose.WithToolsNodeOption(compose.WithToolOption(withAgentToolEnableStreaming(true))))
			}

			var msg Message
			var msgStream MessageStream
			if input.EnableStreaming {
				msgStream, err_ = runnable.Stream(ctx, input, runOpts...)
			} else {
				msg, err_ = runnable.Invoke(ctx, input, runOpts...)
			}

			if err_ == nil {
				if a.outputKey != "" {
					err_ = setOutputToSession(ctx, msg, msgStream, a.outputKey)
					if err_ != nil {
						generator.Send(&AgentEvent{Err: err_})
					}
				} else if msgStream != nil {
					msgStream.Close()
				}
			}

			generator.Close()
		}
```

**File:** adk/chatmodel.go (L918-970)
```go
func (a *ChatModelAgent) Resume(ctx context.Context, info *ResumeInfo, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {
	run := a.buildRunFunc(ctx)

	co := getComposeOptions(opts)
	co = append(co, compose.WithCheckPointID(bridgeCheckpointID))

	if info.InterruptState == nil {
		panic(fmt.Sprintf("ChatModelAgent.Resume: agent '%s' was asked to resume but has no state", a.Name(ctx)))
	}

	stateByte, ok := info.InterruptState.([]byte)
	if !ok {
		panic(fmt.Sprintf("ChatModelAgent.Resume: agent '%s' was asked to resume but has invalid interrupt state type: %T",
			a.Name(ctx), info.InterruptState))
	}

	if info.ResumeData != nil {
		resumeData, ok := info.ResumeData.(*ChatModelAgentResumeData)
		if !ok {
			panic(fmt.Sprintf("ChatModelAgent.Resume: agent '%s' was asked to resume but has invalid resume data type: %T",
				a.Name(ctx), info.ResumeData))
		}

		if resumeData.HistoryModifier != nil {
			co = append(co, compose.WithStateModifier(func(ctx context.Context, path compose.NodePath, state any) error {
				s, ok := state.(*State)
				if !ok {
					return fmt.Errorf("unexpected state type: %T, expected: %T", state, &State{})
				}
				s.Messages = resumeData.HistoryModifier(ctx, s.Messages)
				return nil
			}))
		}
	}

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go func() {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				e := safe.NewPanicErr(panicErr, debug.Stack())
				generator.Send(&AgentEvent{Err: e})
			}

			generator.Close()
		}()

		run(ctx, &AgentInput{EnableStreaming: info.EnableStreaming}, generator,
			newResumeBridgeStore(stateByte), co...)
	}()

	return iterator
}
```

**File:** adk/runner.go (L31-68)
```go
// Runner is the primary entry point for executing an Agent.
// It manages the agent's lifecycle, including starting, resuming, and checkpointing.
type Runner struct {
	// a is the agent to be executed.
	a Agent
	// enableStreaming dictates whether the execution should be in streaming mode.
	enableStreaming bool
	// store is the checkpoint store used to persist agent state upon interruption.
	// If nil, checkpointing is disabled.
	store compose.CheckPointStore
}

type RunnerConfig struct {
	Agent           Agent
	EnableStreaming bool

	CheckPointStore compose.CheckPointStore
}

// ResumeParams contains all parameters needed to resume an execution.
// This struct provides an extensible way to pass resume parameters without
// requiring breaking changes to method signatures.
type ResumeParams struct {
	// Targets contains the addresses of components to be resumed as keys,
	// with their corresponding resume data as values
	Targets map[string]any
	// Future extensible fields can be added here without breaking changes
}

// NewRunner creates a Runner that executes an Agent with optional streaming
// and checkpoint persistence.
func NewRunner(_ context.Context, conf RunnerConfig) *Runner {
	return &Runner{
		enableStreaming: conf.EnableStreaming,
		a:               conf.Agent,
		store:           conf.CheckPointStore,
	}
}
```

**File:** adk/runner.go (L70-98)
```go
// Run starts a new execution of the agent with a given set of messages.
// It returns an iterator that yields agent events as they occur.
// If the Runner was configured with a CheckPointStore, it will automatically save the agent's state
// upon interruption.
func (r *Runner) Run(ctx context.Context, messages []Message,
	opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {
	o := getCommonOptions(nil, opts...)

	fa := toFlowAgent(ctx, r.a)

	input := &AgentInput{
		Messages:        messages,
		EnableStreaming: r.enableStreaming,
	}

	ctx = ctxWithNewRunCtx(ctx, input, o.sharedParentSession)

	AddSessionValues(ctx, o.sessionValues)

	iter := fa.Run(ctx, input, opts...)
	if r.store == nil {
		return iter
	}

	niter, gen := NewAsyncIteratorPair[*AgentEvent]()

	go r.handleIter(ctx, iter, gen, o.checkPointID)
	return niter
}
```

**File:** adk/runner.go (L100-105)
```go
// Query is a convenience method that starts a new execution with a single user query string.
func (r *Runner) Query(ctx context.Context,
	query string, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {

	return r.Run(ctx, []Message{schema.UserMessage(query)}, opts...)
}
```

**File:** adk/runner.go (L107-117)
```go
// Resume continues an interrupted execution from a checkpoint, using an "Implicit Resume All" strategy.
// This method is best for simpler use cases where the act of resuming implies that all previously
// interrupted points should proceed without specific data.
//
// When using this method, all interrupted agents will receive `isResumeFlow = false` when they
// call `GetResumeContext`, as no specific agent was targeted. This is suitable for the "Simple Confirmation"
// pattern where an agent only needs to know `wasInterrupted` is true to continue.
func (r *Runner) Resume(ctx context.Context, checkPointID string, opts ...AgentRunOption) (
	*AsyncIterator[*AgentEvent], error) {
	return r.resume(ctx, checkPointID, nil, opts...)
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

**File:** adk/runner.go (L188-246)
```go
func (r *Runner) handleIter(ctx context.Context, aIter *AsyncIterator[*AgentEvent],
	gen *AsyncGenerator[*AgentEvent], checkPointID *string) {
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			e := safe.NewPanicErr(panicErr, debug.Stack())
			gen.Send(&AgentEvent{Err: e})
		}

		gen.Close()
	}()
	var (
		interruptSignal *core.InterruptSignal
		legacyData      any
	)
	for {
		event, ok := aIter.Next()
		if !ok {
			break
		}

		if event.Action != nil && event.Action.internalInterrupted != nil {
			if interruptSignal != nil {
				// even if multiple interrupt happens, they should be merged into one
				// action by CompositeInterrupt, so here in Runner we must assume at most
				// one interrupt action happens
				panic("multiple interrupt actions should not happen in Runner")
			}
			interruptSignal = event.Action.internalInterrupted
			interruptContexts := core.ToInterruptContexts(interruptSignal, encapsulateAddress)
			event = &AgentEvent{
				AgentName: event.AgentName,
				RunPath:   event.RunPath,
				Output:    event.Output,
				Action: &AgentAction{
					Interrupted: &InterruptInfo{
						Data:              event.Action.Interrupted.Data,
						InterruptContexts: interruptContexts,
					},
					internalInterrupted: interruptSignal,
				},
			}
			legacyData = event.Action.Interrupted.Data

			if checkPointID != nil {
				// save checkpoint first before sending interrupt event,
				// so when end-user receives interrupt event, they can resume from this checkpoint
				err := r.saveCheckPoint(ctx, *checkPointID, &InterruptInfo{
					Data: legacyData,
				}, interruptSignal)
				if err != nil {
					gen.Send(&AgentEvent{Err: fmt.Errorf("failed to save checkpoint: %w", err)})
				}
			}
		}

		gen.Send(event)
	}
}
```

**File:** adk/runctx.go (L91-99)
```go
// GetSessionValues returns all session key-value pairs for the current run.
func GetSessionValues(ctx context.Context) map[string]any {
	session := getSession(ctx)
	if session == nil {
		return map[string]any{}
	}

	return session.getValues()
}
```

**File:** adk/runctx.go (L101-109)
```go
// AddSessionValue sets a single session key-value pair for the current run.
func AddSessionValue(ctx context.Context, key string, value any) {
	session := getSession(ctx)
	if session == nil {
		return
	}

	session.addValue(key, value)
}
```

**File:** adk/runctx.go (L111-119)
```go
// AddSessionValues sets multiple session key-value pairs for the current run.
func AddSessionValues(ctx context.Context, kvs map[string]any) {
	session := getSession(ctx)
	if session == nil {
		return
	}

	session.addValues(kvs)
}
```

**File:** adk/runctx.go (L121-129)
```go
// GetSessionValue retrieves a session value by key and reports whether it exists.
func GetSessionValue(ctx context.Context, key string) (any, bool) {
	session := getSession(ctx)
	if session == nil {
		return nil, false
	}

	return session.getValue(key)
}
```

**File:** adk/interrupt.go (L30-44)
```go
// ResumeInfo holds all the information necessary to resume an interrupted agent execution.
// It is created by the framework and passed to an agent's Resume method.
type ResumeInfo struct {
	// EnableStreaming indicates whether the original execution was in streaming mode.
	EnableStreaming bool

	// Deprecated: use InterruptContexts from the embedded InterruptInfo for user-facing details,
	// and GetInterruptState for internal state retrieval.
	*InterruptInfo

	WasInterrupted bool
	InterruptState any
	IsResumeTarget bool
	ResumeData     any
}
```

**File:** adk/interrupt.go (L56-73)
```go
// Interrupt creates a basic interrupt action.
// This is used when an agent needs to pause its execution to request external input or intervention,
// but does not need to save any internal state to be restored upon resumption.
// The `info` parameter is user-facing data that describes the reason for the interrupt.
func Interrupt(ctx context.Context, info any) *AgentEvent {
	is, err := core.Interrupt(ctx, info, nil, nil,
		core.WithLayerPayload(getRunCtx(ctx).RunPath))
	if err != nil {
		return &AgentEvent{Err: err}
	}

	return &AgentEvent{
		Action: &AgentAction{
			Interrupted:         &InterruptInfo{},
			internalInterrupted: is,
		},
	}
}
```

**File:** adk/interrupt.go (L75-92)
```go
// StatefulInterrupt creates an interrupt action that also saves the agent's internal state.
// This is used when an agent has internal state that must be restored for it to continue correctly.
// The `info` parameter is user-facing data describing the interrupt.
// The `state` parameter is the agent's internal state object, which will be serialized and stored.
func StatefulInterrupt(ctx context.Context, info any, state any) *AgentEvent {
	is, err := core.Interrupt(ctx, info, state, nil,
		core.WithLayerPayload(getRunCtx(ctx).RunPath))
	if err != nil {
		return &AgentEvent{Err: err}
	}

	return &AgentEvent{
		Action: &AgentAction{
			Interrupted:         &InterruptInfo{},
			internalInterrupted: is,
		},
	}
}
```

**File:** adk/interrupt.go (L178-199)
```go
func (r *Runner) loadCheckPoint(ctx context.Context, checkpointID string) (
	context.Context, *runContext, *ResumeInfo, error) {
	data, existed, err := r.store.Get(ctx, checkpointID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get checkpoint from store: %w", err)
	}
	if !existed {
		return nil, nil, nil, fmt.Errorf("checkpoint[%s] not exist", checkpointID)
	}

	s := &serialization{}
	err = gob.NewDecoder(bytes.NewReader(data)).Decode(s)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode checkpoint: %w", err)
	}
	ctx = core.PopulateInterruptState(ctx, s.InterruptID2Address, s.InterruptID2State)

	return ctx, s.RunCtx, &ResumeInfo{
		EnableStreaming: s.EnableStreaming,
		InterruptInfo:   s.Info,
	}, nil
}
```

**File:** adk/interrupt.go (L201-223)
```go
func (r *Runner) saveCheckPoint(
	ctx context.Context,
	key string,
	info *InterruptInfo,
	is *core.InterruptSignal,
) error {
	runCtx := getRunCtx(ctx)

	id2Addr, id2State := core.SignalToPersistenceMaps(is)

	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(&serialization{
		RunCtx:              runCtx,
		Info:                info,
		InterruptID2Address: id2Addr,
		InterruptID2State:   id2State,
		EnableStreaming:     r.enableStreaming,
	})
	if err != nil {
		return fmt.Errorf("failed to encode checkpoint: %w", err)
	}
	return r.store.Set(ctx, key, buf.Bytes())
}
```

**File:** adk/interrupt_test.go (L73-137)
```go
func TestSimpleInterrupt(t *testing.T) {
	data := "hello world"
	agent := &myAgent{
		runFn: func(ctx context.Context, input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent] {
			iter, generator := NewAsyncIteratorPair[*AgentEvent]()
			generator.Send(&AgentEvent{
				Output: &AgentOutput{
					MessageOutput: &MessageVariant{
						IsStreaming: true,
						Message:     nil,
						MessageStream: schema.StreamReaderFromArray([]Message{
							schema.UserMessage("hello "),
							schema.UserMessage("world"),
						}),
					},
				},
			})
			intEvent := Interrupt(ctx, data)
			intEvent.Action.Interrupted.Data = data
			generator.Send(intEvent)
			generator.Close()
			return iter
		},
		resumeFn: func(ctx context.Context, info *ResumeInfo, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {
			assert.True(t, info.WasInterrupted)
			assert.Nil(t, info.InterruptState)
			assert.True(t, info.EnableStreaming)
			assert.Equal(t, data, info.Data)

			assert.True(t, info.IsResumeTarget)
			iter, generator := NewAsyncIteratorPair[*AgentEvent]()
			generator.Close()
			return iter
		},
	}
	store := newMyStore()
	ctx := context.Background()
	runner := NewRunner(ctx, RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
		CheckPointStore: store,
	})
	iter := runner.Query(ctx, "hello world", WithCheckPointID("1"))
	_, ok := iter.Next()
	assert.True(t, ok)
	interruptEvent, ok := iter.Next()
	assert.True(t, ok)
	assert.Equal(t, data, interruptEvent.Action.Interrupted.Data)
	assert.NotEmpty(t, interruptEvent.Action.Interrupted.InterruptContexts[0].ID)
	assert.True(t, interruptEvent.Action.Interrupted.InterruptContexts[0].IsRootCause)
	assert.Equal(t, data, interruptEvent.Action.Interrupted.InterruptContexts[0].Info)
	assert.Equal(t, Address{{Type: AddressSegmentAgent, ID: "myAgent"}},
		interruptEvent.Action.Interrupted.InterruptContexts[0].Address)
	_, ok = iter.Next()
	assert.False(t, ok)

	iter, err := runner.ResumeWithParams(ctx, "1", &ResumeParams{
		Targets: map[string]any{
			interruptEvent.Action.Interrupted.InterruptContexts[0].ID: nil,
		},
	})
	assert.NoError(t, err)
	_, ok = iter.Next()
	assert.False(t, ok)
}
```
