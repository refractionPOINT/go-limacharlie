package limacharlie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// FutureResults is a queue to receive specific events based on investigation_id.
type FutureResults struct {
	queue           chan interface{}
	mu              sync.Mutex
	results         []interface{}
	newResultSignal chan struct{}
	WasReceived     bool
}

// NewFutureResults creates a new FutureResults instance.
func NewFutureResults(bufferSize int) *FutureResults {
	return &FutureResults{
		queue:           make(chan interface{}, bufferSize),
		results:         make([]interface{}, 0),
		newResultSignal: make(chan struct{}, 1),
		WasReceived:     false,
	}
}

// Get retrieves the next result from the future.
func (f *FutureResults) Get() (interface{}, bool) {
	msg, ok := <-f.queue
	return msg, ok
}

// GetWithTimeout retrieves the next result with a timeout.
func (f *FutureResults) GetWithTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case msg, ok := <-f.queue:
		if !ok {
			return nil, errors.New("future results closed")
		}
		return msg, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout waiting for result")
	}
}

// addResult adds a result to the future (internal use).
func (f *FutureResults) addResult(result interface{}) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if this is a CLOUD_NOTIFICATION
	if m, ok := result.(map[string]interface{}); ok {
		if routing, ok := m["routing"].(map[string]interface{}); ok {
			if eventType, ok := routing["event_type"].(string); ok && eventType == "CLOUD_NOTIFICATION" {
				f.WasReceived = true
				// Still send to queue for backward compatibility
				select {
				case f.queue <- result:
					return true
				default:
					return false
				}
			}
		}
	}

	// For non-CLOUD_NOTIFICATION events, accumulate for batch retrieval
	f.results = append(f.results, result)

	// Signal that new results are available (non-blocking)
	select {
	case f.newResultSignal <- struct{}{}:
	default:
	}

	// Also send to queue for backward compatibility with Get()
	select {
	case f.queue <- result:
		return true
	default:
		return false
	}
}

// GetNewResponses retrieves all accumulated results, blocking for up to the specified timeout.
// Returns a slice of results, or an empty slice if the timeout is reached.
// This method clears the accumulated results after retrieval.
func (f *FutureResults) GetNewResponses(timeout time.Duration) []interface{} {
	// Wait for signal or timeout
	select {
	case <-f.newResultSignal:
		// New results are available
		f.mu.Lock()
		defer f.mu.Unlock()

		// Get accumulated results
		ret := f.results
		f.results = make([]interface{}, 0)

		return ret

	case <-time.After(timeout):
		// Timeout reached, return empty slice
		return []interface{}{}
	}
}

// Close closes the future results queue.
func (f *FutureResults) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	close(f.queue)
	close(f.newResultSignal)
}

type futureRegistration struct {
	future *FutureResults
	expiry time.Time
}

// ResumeContext holds WebSocket session context information.
type ResumeContext struct {
	StreamID string `json:"stream_id,omitempty"`
	OID      string `json:"oid,omitempty"`
}

// LiveStreamRequest represents the header sent to the WebSocket server.
type LiveStreamRequest struct {
	OrgID           string        `json:"oid,omitempty"`
	APIKey          string        `json:"api_key,omitempty"`
	JWT             string        `json:"jwt,omitempty"`
	StreamType      string        `json:"type,omitempty"`
	Tag             string        `json:"tag,omitempty"`
	Category        string        `json:"cat,omitempty"`
	InvestigationID string        `json:"inv_id,omitempty"`
	SensorID        string        `json:"sid,omitempty"`
	UserID          string        `json:"uid,omitempty"`
	ResumeContext   ResumeContext `json:"resume_context,omitempty"`
}

const (
	// Default timeout for WebSocket operations
	defaultWebSocketTimeout = 5 * time.Second
	// Default keep-alive interval
	cloudKeepAliveInterval = 60 * time.Second
	cloudTimeout           = (cloudKeepAliveInterval * 2) + 1*time.Second
)

// Spout is a listener object to receive data (Events, Detects or Audit) from a limacharlie.io Organization.
type Spout struct {
	org              *Organization
	dataType         string
	isParse          bool
	maxBuffer        int
	invID            string
	tag              string
	cat              string
	sid              string
	userID           string
	extraParams      map[string]interface{}
	dropped          int64
	isStop           bool
	queue            chan interface{}
	conn             *websocket.Conn
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	futures          map[string]*futureRegistration
	futuresMu        sync.RWMutex
	reconnectEnabled bool
}

// NewSpout creates a new Spout instance to receive data from limacharlie.io.
func NewSpout(org *Organization, dataType string, opts ...SpoutOption) (*Spout, error) {
	if dataType != "event" && dataType != "detect" && dataType != "audit" && dataType != "deployment" && dataType != "billing" {
		return nil, fmt.Errorf("invalid data type: %s", dataType)
	}

	sp := &Spout{
		org:              org,
		dataType:         dataType,
		isParse:          true,
		maxBuffer:        1024,
		futures:          make(map[string]*futureRegistration),
		reconnectEnabled: true,
	}

	// Apply options
	for _, opt := range opts {
		opt(sp)
	}

	sp.queue = make(chan interface{}, sp.maxBuffer)
	sp.ctx, sp.cancel = context.WithCancel(context.Background())

	return sp, nil
}

// SpoutOption is a function that configures a Spout.
type SpoutOption func(*Spout)

// WithParse sets whether to parse the data as JSON.
func WithParse(parse bool) SpoutOption {
	return func(s *Spout) {
		s.isParse = parse
	}
}

// WithMaxBuffer sets the maximum number of messages to buffer.
func WithMaxBuffer(max int) SpoutOption {
	return func(s *Spout) {
		s.maxBuffer = max
	}
}

// WithInvestigationID sets the investigation ID filter.
func WithInvestigationID(invID string) SpoutOption {
	return func(s *Spout) {
		s.invID = invID
	}
}

// WithTag sets the tag filter.
func WithTag(tag string) SpoutOption {
	return func(s *Spout) {
		s.tag = tag
	}
}

// WithCategory sets the category filter.
func WithCategory(cat string) SpoutOption {
	return func(s *Spout) {
		s.cat = cat
	}
}

// WithSensorID sets the sensor ID filter.
func WithSensorID(sid string) SpoutOption {
	return func(s *Spout) {
		s.sid = sid
	}
}

// WithUserID sets the user ID.
func WithUserID(uid string) SpoutOption {
	return func(s *Spout) {
		s.userID = uid
	}
}

// WithReconnect enables or disables automatic reconnection.
func WithReconnect(enabled bool) SpoutOption {
	return func(s *Spout) {
		s.reconnectEnabled = enabled
	}
}

// WithExtraParams sets additional parameters to be sent with the spout request.
func WithExtraParams(params map[string]interface{}) SpoutOption {
	return func(s *Spout) {
		s.extraParams = params
	}
}

// Start begins receiving data from limacharlie.io.
func (s *Spout) Start() error {
	// Create WebSocket connection
	header := LiveStreamRequest{
		OrgID:           s.org.GetOID(),
		APIKey:          s.org.client.options.APIKey,
		JWT:             s.org.GetCurrentJWT(),
		StreamType:      s.dataType,
		Tag:             s.tag,
		Category:        s.cat,
		InvestigationID: s.invID,
		SensorID:        s.sid,
		UserID:          s.userID,
		ResumeContext: ResumeContext{
			OID: s.org.GetOID(),
		},
	}

	// Connect to WebSocket
	conn, err := s.connectWebSocket(header)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	s.conn = conn

	// Create a channel to signal connection success
	connected := make(chan bool, 1)

	// Start the main message reading goroutine immediately with the connected signal
	// This ensures there's only ever ONE goroutine reading from the WebSocket,
	// avoiding race conditions and the "unexpected EOF" errors
	go s.readMessages(connected)

	// Start the futures cleanup goroutine
	go s.cleanupFutures()

	// Wait for connection confirmation or timeout
	select {
	case <-connected:
		s.org.logger.Info("WebSocket connection confirmed")
		return nil
	case <-time.After(10 * time.Second):
		s.Shutdown()
		return fmt.Errorf("timeout waiting for connection confirmation")
	}
}

// RegisterFutureResults registers a FutureResults to receive events with a specific investigation_id.
// The tracking_id should be the full investigation_id value to match (including any custom tracking after "/").
func (s *Spout) RegisterFutureResults(trackingID string, future *FutureResults, ttl time.Duration) {
	s.futuresMu.Lock()
	defer s.futuresMu.Unlock()
	s.futures[trackingID] = &futureRegistration{
		future: future,
		expiry: time.Now().Add(ttl),
	}
}

// cleanupFutures periodically removes expired future registrations.
func (s *Spout) cleanupFutures() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.futuresMu.Lock()
			now := time.Now()
			for trackingID, reg := range s.futures {
				if now.After(reg.expiry) {
					reg.future.Close()
					delete(s.futures, trackingID)
				}
			}
			s.futuresMu.Unlock()
		}
	}
}

// connectWebSocket establishes a WebSocket connection to the server.
func (s *Spout) connectWebSocket(header LiveStreamRequest) (*websocket.Conn, error) {
	// Create WebSocket dialer
	dialer := websocket.Dialer{
		HandshakeTimeout: defaultWebSocketTimeout,
	}

	// Connect to WebSocket
	conn, _, err := dialer.Dial("wss://stream.limacharlie.io/ws", nil)
	if err != nil {
		return nil, err
	}

	// If there are extra params, merge them with the header
	if len(s.extraParams) > 0 {
		// Marshal header to JSON
		headerBytes, err := json.Marshal(header)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to marshal header: %v", err)
		}

		// Unmarshal into a map
		var headerMap map[string]interface{}
		if err := json.Unmarshal(headerBytes, &headerMap); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to unmarshal header: %v", err)
		}

		// Add extra params to the map
		for k, v := range s.extraParams {
			headerMap[k] = v
		}

		// Send merged map
		if err := conn.WriteJSON(headerMap); err != nil {
			conn.Close()
			return nil, err
		}
	} else {
		// Send header as-is
		if err := conn.WriteJSON(header); err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

// readMessages continuously reads messages from the WebSocket connection.
// It includes auto-reconnect logic similar to the Python implementation.
// If connectedSignal is provided, it will be signaled when the initial "connected" trace message is received.
func (s *Spout) readMessages(connectedSignal chan bool) {
	defer s.Shutdown()
	s.org.logger.Info("Starting to read messages")

	isConnected := connectedSignal == nil // If no signal channel, assume already connected

	// Outer loop for auto-reconnect
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Inner loop for reading messages from current connection
		func() {
			defer func() {
				if r := recover(); r != nil {
					s.org.logger.Info(fmt.Sprintf("panic in readMessages: %v", r))
				}
			}()

			for {
				select {
				case <-s.ctx.Done():
					return
				default:
					// Set read deadline
					if err := s.conn.SetReadDeadline(time.Now().Add(cloudTimeout)); err != nil {
						s.org.logger.Info(fmt.Sprintf("error setting read deadline: %v", err))
						return
					}

					// Read message
					_, message, err := s.conn.ReadMessage()
					if err != nil {
						if !websocket.IsCloseError(err, websocket.CloseNormalClosure) && !s.isStop {
							s.org.logger.Info(fmt.Sprintf("stream closed: %v", err))
						}
						return
					}

					// Check for connected trace message (only if we haven't signaled yet)
					if !isConnected {
						var data map[string]interface{}
						if err := json.Unmarshal(message, &data); err == nil {
							if trace, ok := data["__trace"].(string); ok && trace == "connected" {
								s.org.logger.Info("Received connected trace message")
								isConnected = true
								if connectedSignal != nil {
									connectedSignal <- true
									close(connectedSignal)
								}
								continue // Skip processing this trace message
							}
						}
					}

					// Process message
					if err := s.processMessage(message); err != nil {
						s.org.logger.Info(fmt.Sprintf("error processing message: %v", err))
						continue
					}
				}
			}
		}()

		// Check if we should reconnect
		s.mu.Lock()
		shouldReconnect := s.reconnectEnabled && !s.isStop
		s.mu.Unlock()

		if !shouldReconnect {
			return
		}

		// Attempt to reconnect
		s.org.logger.Info("Attempting to reconnect...")
		if err := s.reconnect(); err != nil {
			s.org.logger.Info(fmt.Sprintf("reconnect failed: %v, retrying in 5 seconds...", err))
			time.Sleep(5 * time.Second)
		} else {
			s.org.logger.Info("Reconnected successfully")
		}
	}
}

// reconnect re-establishes the WebSocket connection.
func (s *Spout) reconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close old connection if it exists
	if s.conn != nil {
		s.conn.Close()
	}

	// Create new connection
	header := LiveStreamRequest{
		OrgID:           s.org.GetOID(),
		APIKey:          s.org.client.options.APIKey,
		JWT:             s.org.GetCurrentJWT(),
		StreamType:      s.dataType,
		Tag:             s.tag,
		Category:        s.cat,
		InvestigationID: s.invID,
		SensorID:        s.sid,
		UserID:          s.userID,
		ResumeContext: ResumeContext{
			OID: s.org.GetOID(),
		},
	}

	conn, err := s.connectWebSocket(header)
	if err != nil {
		return fmt.Errorf("failed to reconnect: %v", err)
	}

	s.conn = conn
	return nil
}

// processMessage handles an incoming message.
func (s *Spout) processMessage(message []byte) error {
	if !s.isParse {
		select {
		case s.queue <- message:
			return nil
		case <-time.After(10 * time.Second):
			atomic.AddInt64(&s.dropped, 1)
			return errors.New("queue full after timeout")
		}
	}

	var data interface{}
	if err := json.Unmarshal(message, &data); err != nil {
		return fmt.Errorf("error unmarshaling message: %v", err)
	}

	// Check for trace messages
	if m, ok := data.(map[string]interface{}); ok {
		if trace, ok := m["__trace"].(string); ok {
			if trace == "dropped" {
				if n, ok := m["n"].(float64); ok {
					atomic.AddInt64(&s.dropped, int64(n))
				}
			}
			return nil
		}

		// Check if this message should be routed to a registered FutureResults
		if routing, ok := m["routing"].(map[string]interface{}); ok {
			if invID, ok := routing["investigation_id"].(string); ok && invID != "" {
				s.org.logger.Info(fmt.Sprintf("[Spout] Message with investigation_id: %s", invID))
				s.futuresMu.RLock()
				if reg, exists := s.futures[invID]; exists {
					s.futuresMu.RUnlock()
					s.org.logger.Info(fmt.Sprintf("[Spout] Routing to registered FutureResults for: %s", invID))
					// Try to add to the future's queue
					if !reg.future.addResult(data) {
						atomic.AddInt64(&s.dropped, 1)
						return errors.New("future queue full")
					}
					return nil
				}
				s.futuresMu.RUnlock()
				s.org.logger.Info(fmt.Sprintf("[Spout] No registered FutureResults for: %s, adding to global queue", invID))
			}
		}
	}

	s.org.logger.Info(fmt.Sprintf("[Spout] Adding message to global queue"))

	select {
	case s.queue <- data:
		return nil
	case <-time.After(10 * time.Second):
		atomic.AddInt64(&s.dropped, 1)
		return errors.New("queue full after timeout")
	}
}

// Get returns the next message from the queue.
func (s *Spout) Get() (interface{}, error) {
	select {
	case msg := <-s.queue:
		return msg, nil
	case <-s.ctx.Done():
		return nil, errors.New("spout stopped")
	}
}

// GetWithTimeout returns the next message from the queue with a timeout.
func (s *Spout) GetWithTimeout(timeout time.Duration) (interface{}, error) {
	select {
	case msg := <-s.queue:
		return msg, nil
	case <-s.ctx.Done():
		return nil, errors.New("spout stopped")
	case <-time.After(timeout):
		return nil, errors.New("timeout waiting for message")
	}
}

// GetDropped returns the number of dropped messages.
func (s *Spout) GetDropped() int64 {
	return atomic.LoadInt64(&s.dropped)
}

// ResetDroppedCounter resets the dropped message counter.
func (s *Spout) ResetDroppedCounter() {
	atomic.StoreInt64(&s.dropped, 0)
}

// Shutdown stops the Spout and closes the connection.
func (s *Spout) Shutdown() {
	s.mu.Lock()
	if s.isStop {
		s.mu.Unlock()
		return
	}
	s.isStop = true
	s.mu.Unlock()
	s.org.logger.Info("Shutting down Spout")

	s.cancel()
	if s.conn != nil {
		s.conn.Close()
	}

	// Close all registered futures
	s.futuresMu.Lock()
	for _, reg := range s.futures {
		reg.future.Close()
	}
	s.futures = make(map[string]*futureRegistration)
	s.futuresMu.Unlock()

	close(s.queue)
}
