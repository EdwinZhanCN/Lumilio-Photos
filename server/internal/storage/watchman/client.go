package watchman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Response represents a generic watchman JSON response.
type Response map[string]any

// WatchProjectResult contains watch root and optional relative path.
type WatchProjectResult struct {
	Watch        string
	RelativePath string
}

// FileEvent represents a watched file entry from watchman query/subscribe.
type FileEvent struct {
	Name    string
	Exists  bool
	New     bool
	Type    string
	Size    int64
	MTimeMs int64
}

// QueryResult contains parsed query/subscription file payload.
type QueryResult struct {
	Clock           string
	IsFreshInstance bool
	Files           []FileEvent
}

// Client is a lightweight JSON-over-socket watchman client.
type Client struct {
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	mu   sync.Mutex
}

// Dial opens a unix socket connection to watchman.
func Dial(ctx context.Context, socketPath string) (*Client, error) {
	if strings.TrimSpace(socketPath) == "" {
		return nil, fmt.Errorf("watchman socket path is empty")
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial watchman socket %q: %w", socketPath, err)
	}

	dec := json.NewDecoder(conn)
	dec.UseNumber()

	return &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  dec,
	}, nil
}

// Close closes the underlying socket connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Command sends a command and waits for the response.
func (c *Client) Command(ctx context.Context, command []any) (Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c == nil {
		return nil, fmt.Errorf("watchman client is nil")
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := c.conn.SetDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set deadline: %w", err)
		}
		defer c.conn.SetDeadline(time.Time{})
	}

	if err := c.enc.Encode(command); err != nil {
		return nil, fmt.Errorf("write command: %w", err)
	}

	// watchman can interleave unilateral notifications; skip them here.
	for i := 0; i < 64; i++ {
		msg, err := c.readRaw()
		if err != nil {
			return nil, err
		}
		if _, ok := msg["subscription"]; ok {
			continue
		}
		if err := parseWatchmanError(msg); err != nil {
			return nil, err
		}
		return msg, nil
	}

	return nil, fmt.Errorf("did not receive command response after 64 messages")
}

// ReadMessage reads a single message with a read timeout.
func (c *Client) ReadMessage(timeout time.Duration) (Response, error) {
	if c == nil {
		return nil, fmt.Errorf("watchman client is nil")
	}
	if timeout > 0 {
		if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return nil, fmt.Errorf("set read deadline: %w", err)
		}
		defer c.conn.SetReadDeadline(time.Time{})
	}
	return c.readRaw()
}

func (c *Client) readRaw() (Response, error) {
	var msg Response
	if err := c.dec.Decode(&msg); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if msg == nil {
		return nil, fmt.Errorf("empty response from watchman")
	}
	return msg, nil
}

// IsTimeoutError checks whether the error is a network timeout.
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// Version verifies socket connectivity and protocol response.
func (c *Client) Version(ctx context.Context) (string, error) {
	resp, err := c.Command(ctx, []any{"version"})
	if err != nil {
		return "", err
	}
	version, _ := getString(resp, "version")
	return version, nil
}

// WatchProject resolves the canonical watch root for a path.
func (c *Client) WatchProject(ctx context.Context, path string) (*WatchProjectResult, error) {
	resp, err := c.Command(ctx, []any{"watch-project", path})
	if err != nil {
		return nil, err
	}

	watch, ok := getString(resp, "watch")
	if !ok || watch == "" {
		return nil, fmt.Errorf("watch-project response missing watch root")
	}
	relativePath, _ := getString(resp, "relative_path")

	return &WatchProjectResult{
		Watch:        watch,
		RelativePath: relativePath,
	}, nil
}

// Clock returns current watch clock token.
func (c *Client) Clock(ctx context.Context, watchRoot string) (string, error) {
	resp, err := c.Command(ctx, []any{"clock", watchRoot})
	if err != nil {
		return "", err
	}
	clock, ok := getString(resp, "clock")
	if !ok || clock == "" {
		return "", fmt.Errorf("clock response missing clock token")
	}
	return clock, nil
}

// Query runs a one-shot file query.
func (c *Client) Query(ctx context.Context, watchRoot string, options map[string]any) (*QueryResult, error) {
	resp, err := c.Command(ctx, []any{"query", watchRoot, options})
	if err != nil {
		return nil, err
	}
	return ParseQueryResult(resp)
}

// Subscribe creates a persistent subscription.
func (c *Client) Subscribe(ctx context.Context, watchRoot, name string, options map[string]any) (string, error) {
	resp, err := c.Command(ctx, []any{"subscribe", watchRoot, name, options})
	if err != nil {
		return "", err
	}
	clock, _ := getString(resp, "clock")
	return clock, nil
}

// ParseQueryResult parses watchman query/subscription payload to typed events.
func ParseQueryResult(resp Response) (*QueryResult, error) {
	if err := parseWatchmanError(resp); err != nil {
		return nil, err
	}

	result := &QueryResult{}
	if clock, ok := getString(resp, "clock"); ok {
		result.Clock = clock
	}
	if fresh, ok := getBool(resp, "is_fresh_instance"); ok {
		result.IsFreshInstance = fresh
	}

	filesRaw, ok := resp["files"]
	if !ok {
		return result, nil
	}

	filesAny, ok := filesRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("watchman files payload has unexpected type %T", filesRaw)
	}

	result.Files = make([]FileEvent, 0, len(filesAny))
	for _, entry := range filesAny {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := getString(m, "name")
		if name == "" {
			continue
		}
		exists, existsOK := getBool(m, "exists")
		if !existsOK {
			// query responses may omit exists; treat as existing regular file.
			exists = true
		}
		isNew, _ := getBool(m, "new")
		fileType, _ := getString(m, "type")
		size, _ := getInt64(m, "size")
		mtimeMs, _ := getInt64(m, "mtime_ms")

		result.Files = append(result.Files, FileEvent{
			Name:    name,
			Exists:  exists,
			New:     isNew,
			Type:    fileType,
			Size:    size,
			MTimeMs: mtimeMs,
		})
	}

	return result, nil
}

func parseWatchmanError(resp Response) error {
	raw, ok := resp["error"]
	if !ok {
		return nil
	}
	msg := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if msg == "" {
		return fmt.Errorf("watchman returned unknown error")
	}
	return fmt.Errorf("watchman error: %s", msg)
}

func getString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func getBool(m map[string]any, key string) (bool, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func getInt64(m map[string]any, key string) (int64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}

	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	case json.Number:
		parsed, err := n.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
