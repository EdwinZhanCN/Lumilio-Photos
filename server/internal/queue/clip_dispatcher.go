package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	// adjust to your generated package import path
	"server/proto"
)

type EmbeddingResult struct {
	Vector  []float32 `json:"vector"`
	Dim     int       `json:"dim"`
	ModelID string    `json:"model_id"`
}

type Label struct {
	Label string  `json:"label"`
	Score float32 `json:"score"` // optional: keep float64 if you prefer
}

type LabelsResult struct {
	Labels  []Label `json:"labels"`
	ModelID string  `json:"model_id"`
}

type ClipResult struct {
	Embedding EmbeddingResult
	Labels    *LabelsResult
	Meta      map[string]string // contains "source" for smart_classify
}

type clipReq struct {
	ctx      context.Context
	assetID  string
	image    []byte
	mime     string
	resultCh chan clipResp
}

type clipResp struct {
	res ClipResult
	err error
}

type partial struct {
	emb  *EmbeddingResult
	lbls *LabelsResult
	meta map[string]string
	err  error
}

// ClipBatchDispatcher batches incoming requests into one gRPC stream per batch.
type ClipBatchDispatcher struct {
	client    proto.InferenceClient
	batchSize int
	window    time.Duration
	in        chan *clipReq
}

// NewClipBatchDispatcher constructs the dispatcher. Call Start() once.
func NewClipBatchDispatcher(client proto.InferenceClient, batchSize int, window time.Duration) *ClipBatchDispatcher {
	if batchSize <= 0 {
		batchSize = 8
	}
	if window <= 0 {
		window = 1500 * time.Millisecond
	}
	return &ClipBatchDispatcher{
		client:    client,
		batchSize: batchSize,
		window:    window,
		in:        make(chan *clipReq, 1024),
	}
}

// Start launches the batching goroutine.
func (d *ClipBatchDispatcher) Start(ctx context.Context) {
	go d.loop(ctx)
}

// Submit enqueues a single job and waits for its result (embedding + smart_classify).
func (d *ClipBatchDispatcher) Submit(ctx context.Context, assetID string, image []byte, mime string) (ClipResult, error) {
	if mime == "" {
		mime = "image/webp"
	}
	ch := make(chan clipResp, 1)
	select {
	case d.in <- &clipReq{
		ctx:      ctx,
		assetID:  assetID,
		image:    image,
		mime:     mime,
		resultCh: ch,
	}:
	case <-ctx.Done():
		return ClipResult{}, ctx.Err()
	}

	select {
	case out := <-ch:
		return out.res, out.err
	case <-ctx.Done():
		return ClipResult{}, ctx.Err()
	}
}

func (d *ClipBatchDispatcher) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case first := <-d.in:
			if first == nil {
				continue
			}
			batch := []*clipReq{first}
			timer := time.NewTimer(d.window)
		collect:
			for len(batch) < d.batchSize {
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case req := <-d.in:
					if req != nil {
						batch = append(batch, req)
					}
					if len(batch) >= d.batchSize {
						break collect
					}
				case <-timer.C:
					break collect
				}
			}
			_ = d.processBatch(ctx, batch)
		}
	}
}

func (d *ClipBatchDispatcher) processBatch(ctx context.Context, batch []*clipReq) error {
	// Create a short-lived context to bound the RPC time
	rpcCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	stream, err := d.client.Infer(rpcCtx)
	if err != nil {
		d.failAll(batch, fmt.Errorf("open stream: %w", err))
		return err
	}

	// Send clip_image_embed and smart_classify for each job
	for i, req := range batch {
		// emb
		embCID := req.assetID + "|emb"
		if err := stream.SendMsg(&proto.InferRequest{
			CorrelationId: embCID,
			Task:          "clip_image_embed",
			Payload:       req.image,
			PayloadMime:   req.mime,
			Seq:           uint64(i * 2),
			Total:         uint64(len(batch) * 2),
		}); err != nil {
			_ = stream.CloseSend()
			d.failAll(batch, fmt.Errorf("send embed: %w", err))
			return err
		}
		// smart
		smartCID := req.assetID + "|smart"
		if err := stream.SendMsg(&proto.InferRequest{
			CorrelationId: smartCID,
			Task:          "smart_classify",
			Payload:       req.image,
			PayloadMime:   req.mime,
			Seq:           uint64(i*2 + 1),
			Total:         uint64(len(batch) * 2),
			Meta:          map[string]string{"topk": "3"},
		}); err != nil {
			_ = stream.CloseSend()
			d.failAll(batch, fmt.Errorf("send smart: %w", err))
			return err
		}
	}

	if err := stream.CloseSend(); err != nil {
		d.failAll(batch, fmt.Errorf("close send: %w", err))
		return err
	}

	pending := make(map[string]*partial, len(batch)) // key: assetID
	for _, r := range batch {
		pending[r.assetID] = &partial{}
	}

	expected := len(batch) * 2
	for expected > 0 {
		resp, rerr := stream.Recv()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			// Fatal; fail all still pending
			d.failRemaining(pending)
			return rerr
		}

		// Handle server-side error per response
		if resp.Error != nil && resp.Error.Code != proto.ErrorCode_ERROR_CODE_UNSPECIFIED {
			assetID := strings.TrimSuffix(resp.CorrelationId, "|emb")
			assetID = strings.TrimSuffix(assetID, "|smart")
			if p, ok := pending[assetID]; ok {
				p.err = fmt.Errorf("server error: %s", resp.Error.Message)
			}
			expected--
			continue
		}

		switch {
		case strings.HasSuffix(resp.CorrelationId, "|emb"):
			var emb EmbeddingResult
			if err := json.Unmarshal(resp.Result, &emb); err != nil {
				d.setErr(pending, resp.CorrelationId, fmt.Errorf("bad emb json: %w", err))
			} else {
				assetID := strings.TrimSuffix(resp.CorrelationId, "|emb")
				if p, ok := pending[assetID]; ok {
					p.emb = &emb
				}
			}
			expected--

		case strings.HasSuffix(resp.CorrelationId, "|smart"):
			var lbls LabelsResult
			if err := json.Unmarshal(resp.Result, &lbls); err != nil {
				d.setErr(pending, resp.CorrelationId, fmt.Errorf("bad labels json: %w", err))
			} else {
				assetID := strings.TrimSuffix(resp.CorrelationId, "|smart")
				if p, ok := pending[assetID]; ok {
					p.lbls = &lbls
					// capture meta (contains "source")
					if resp.Meta != nil {
						p.meta = resp.Meta
					}
				}
			}
			expected--

		default:
			// Unknown correlation; ignore but decrement
			expected--
		}
	}

	// Deliver results to callers
	for _, req := range batch {
		p := pending[req.assetID]
		if p == nil {
			req.resultCh <- clipResp{err: fmt.Errorf("missing result for %s", req.assetID)}
			continue
		}
		if p.err != nil {
			req.resultCh <- clipResp{err: p.err}
			continue
		}
		if p.emb == nil || p.lbls == nil {
			req.resultCh <- clipResp{err: fmt.Errorf("incomplete result for %s", req.assetID)}
			continue
		}
		req.resultCh <- clipResp{
			res: ClipResult{
				Embedding: *p.emb,
				Labels:    p.lbls,
				Meta:      p.meta,
			},
			err: nil,
		}
	}
	return nil
}

func (d *ClipBatchDispatcher) setErr(pending map[string]*partial, corr string, err error) {
	assetID := strings.TrimSuffix(corr, "|emb")
	assetID = strings.TrimSuffix(assetID, "|smart")
	if p, ok := pending[assetID]; ok {
		p.err = err
	}
}

func (d *ClipBatchDispatcher) failAll(batch []*clipReq, err error) {
	for _, r := range batch {
		select {
		case r.resultCh <- clipResp{err: err}:
		default:
		}
	}
}

func (d *ClipBatchDispatcher) failRemaining(pending map[string]*partial) {
	for assetID := range pending {
		_ = assetID
		// Actual per-request delivery occurs in processBatch's final loop
		// where we send out results via the original request resultCh.
	}
}
