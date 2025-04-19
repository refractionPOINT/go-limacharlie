package limacharlie

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// LiveStreamRequest represents the header sent to the WebSocket server.
type LiveStreamRequest struct {
	OrgID           string `json:"oid,omitempty"`
	APIKey          string `json:"api_key,omitempty"`
	JWT             string `json:"jwt,omitempty"`
	StreamType      string `json:"type,omitempty"`
	Tag             string `json:"tag,omitempty"`
	Category        string `json:"cat,omitempty"`
	InvestigationID string `json:"inv_id,omitempty"`
	SensorID        string `json:"sid,omitempty"`
	UserID          string `json:"uid,omitempty"`
}

// Manager represents the limacharlie.io manager.
type Manager struct {
	oid          string
	secretAPIKey string
	jwt          string
	Log          func(format string, args ...interface{})
}

const (
	// Default timeout for WebSocket operations
	defaultWebSocketTimeout = 5 * time.Second
	// Default keep-alive interval
	defaultKeepAliveInterval = 25 * time.Second
)

// Spout is a listener object to receive data (Events, Detects or Audit) from a limacharlie.io Organization.
type Spout struct {
	man       *Manager
	dataType  string
	isParse   bool
	maxBuffer int
	invID     string
	tag       string
	cat       string
	sid       string
	userID    string
	dropped   int64
	isStop    bool
	queue     chan interface{}
	conn      *websocket.Conn
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewSpout creates a new Spout instance to receive data from limacharlie.io.
func NewSpout(man *Manager, dataType string, opts ...SpoutOption) (*Spout, error) {
	if dataType != "event" && dataType != "detect" && dataType != "audit" && dataType != "deployment" && dataType != "billing" {
		return nil, fmt.Errorf("invalid data type: %s", dataType)
	}

	sp := &Spout{
		man:       man,
		dataType:  dataType,
		isParse:   true,
		maxBuffer: 1024,
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

// Start begins receiving data from limacharlie.io.
func (s *Spout) Start() error {
	// Create WebSocket connection
	header := LiveStreamRequest{
		OrgID:           s.man.oid,
		APIKey:          s.man.secretAPIKey,
		JWT:             s.man.jwt,
		StreamType:      s.dataType,
		Tag:             s.tag,
		Category:        s.cat,
		InvestigationID: s.invID,
		SensorID:        s.sid,
		UserID:          s.userID,
	}

	// Connect to WebSocket
	conn, err := s.connectWebSocket(header)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	s.conn = conn

	// Start reading messages
	go s.readMessages()

	return nil
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

	// Send header
	if err := conn.WriteJSON(header); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// readMessages continuously reads messages from the WebSocket connection.
func (s *Spout) readMessages() {
	defer s.conn.Close()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Set read deadline
			if err := s.conn.SetReadDeadline(time.Now().Add(defaultWebSocketTimeout)); err != nil {
				s.man.Log("error setting read deadline: %v", err)
				return
			}

			// Read message
			_, message, err := s.conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					s.man.Log("error reading message: %v", err)
				}
				return
			}

			// Process message
			if err := s.processMessage(message); err != nil {
				s.man.Log("error processing message: %v", err)
				continue
			}
		}
	}
}

// processMessage handles an incoming message.
func (s *Spout) processMessage(message []byte) error {
	if !s.isParse {
		select {
		case s.queue <- message:
			return nil
		default:
			s.mu.Lock()
			s.dropped++
			s.mu.Unlock()
			return errors.New("queue full")
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
					s.mu.Lock()
					s.dropped += int64(n)
					s.mu.Unlock()
				}
			}
			return nil
		}
	}

	select {
	case s.queue <- data:
		return nil
	default:
		s.mu.Lock()
		s.dropped++
		s.mu.Unlock()
		return errors.New("queue full")
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

// GetDropped returns the number of dropped messages.
func (s *Spout) GetDropped() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// ResetDroppedCounter resets the dropped message counter.
func (s *Spout) ResetDroppedCounter() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropped = 0
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

	s.cancel()
	if s.conn != nil {
		s.conn.Close()
	}
}
