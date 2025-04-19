package limacharlie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestOrganization() *Organization {
	return &Organization{
		client: &Client{
			options: ClientOptions{
				OID:    "test-oid",
				APIKey: "test-key",
			},
			logger: &testLogger{},
		},
		logger: &testLogger{},
	}
}

type testLogger struct{}

func (l *testLogger) Info(format string)  {}
func (l *testLogger) Debug(format string) {}
func (l *testLogger) Error(format string) {}
func (l *testLogger) Fatal(format string) {}
func (l *testLogger) Trace(format string) {}
func (l *testLogger) Warn(format string)  {}

func TestNewSpout(t *testing.T) {
	tests := []struct {
		name      string
		dataType  string
		opts      []SpoutOption
		wantErr   bool
		checkFunc func(*testing.T, *Spout)
	}{
		{
			name:     "valid event type",
			dataType: "event",
			opts:     []SpoutOption{},
			wantErr:  false,
			checkFunc: func(t *testing.T, s *Spout) {
				assert.Equal(t, "event", s.dataType)
				assert.True(t, s.isParse)
				assert.Equal(t, 1024, s.maxBuffer)
			},
		},
		{
			name:     "valid detect type",
			dataType: "detect",
			opts:     []SpoutOption{},
			wantErr:  false,
		},
		{
			name:     "valid audit type",
			dataType: "audit",
			opts:     []SpoutOption{},
			wantErr:  false,
		},
		{
			name:     "invalid type",
			dataType: "invalid",
			opts:     []SpoutOption{},
			wantErr:  true,
		},
		{
			name:     "with options",
			dataType: "event",
			opts: []SpoutOption{
				WithParse(false),
				WithMaxBuffer(2048),
				WithTag("test-tag"),
				WithCategory("test-cat"),
				WithInvestigationID("test-inv"),
				WithSensorID("test-sid"),
				WithUserID("test-uid"),
			},
			wantErr: false,
			checkFunc: func(t *testing.T, s *Spout) {
				assert.False(t, s.isParse)
				assert.Equal(t, 2048, s.maxBuffer)
				assert.Equal(t, "test-tag", s.tag)
				assert.Equal(t, "test-cat", s.cat)
				assert.Equal(t, "test-inv", s.invID)
				assert.Equal(t, "test-sid", s.sid)
				assert.Equal(t, "test-uid", s.userID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := createTestOrganization()

			got, err := NewSpout(org, tt.dataType, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Equal(t, org, got.org)
			assert.Equal(t, tt.dataType, got.dataType)

			if tt.checkFunc != nil {
				tt.checkFunc(t, got)
			}
		})
	}
}

func TestSpout_StartAndShutdown(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("failed to upgrade connection: %v", err)
		}
		defer conn.Close()

		// Read the header
		var header LiveStreamRequest
		if err := conn.ReadJSON(&header); err != nil {
			t.Fatalf("failed to read header: %v", err)
		}

		// Send some test messages
		conn.WriteJSON(map[string]interface{}{"test": "message1"})
		conn.WriteJSON(map[string]interface{}{"test": "message2"})
	}))
	defer server.Close()

	// Create a Spout with the test server URL
	org := createTestOrganization()

	spout, err := NewSpout(org, "event")
	require.NoError(t, err)

	// Create a test connection
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial("ws"+server.URL[4:], nil)
	require.NoError(t, err)

	// Manually set the connection for testing
	spout.conn = conn

	// Start reading messages in a goroutine
	go spout.readMessages()

	// Get messages
	msg1, err := spout.Get()
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"test": "message1"}, msg1)

	msg2, err := spout.Get()
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"test": "message2"}, msg2)

	// Shutdown
	spout.Shutdown()

	// Verify shutdown
	_, err = spout.Get()
	assert.Error(t, err)
	assert.Equal(t, "spout stopped", err.Error())
}

func TestSpout_DroppedMessages(t *testing.T) {
	org := createTestOrganization()

	spout, err := NewSpout(org, "event", WithMaxBuffer(1))
	require.NoError(t, err)

	// Create a test message
	msg := map[string]interface{}{"test": "message"}

	// Fill the buffer
	spout.queue <- msg

	// Try to add another message (should be dropped)
	err = spout.processMessage([]byte(`{"test":"message2"}`))
	assert.Error(t, err)
	assert.Equal(t, "queue full", err.Error())

	// Verify dropped count
	assert.Equal(t, int64(1), spout.GetDropped())

	// Reset counter
	spout.ResetDroppedCounter()
	assert.Equal(t, int64(0), spout.GetDropped())
}

func TestSpout_TraceMessages(t *testing.T) {
	org := createTestOrganization()

	spout, err := NewSpout(org, "event")
	require.NoError(t, err)

	// Test dropped trace message
	droppedMsg := map[string]interface{}{
		"__trace": "dropped",
		"n":       5,
	}
	droppedJSON, _ := json.Marshal(droppedMsg)

	err = spout.processMessage(droppedJSON)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), spout.GetDropped())

	// Test other trace message
	otherTraceMsg := map[string]interface{}{
		"__trace": "other",
	}
	otherTraceJSON, _ := json.Marshal(otherTraceMsg)

	err = spout.processMessage(otherTraceJSON)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), spout.GetDropped()) // Should not increment
}

func TestSpout_UnparseableMessages(t *testing.T) {
	org := createTestOrganization()

	spout, err := NewSpout(org, "event")
	require.NoError(t, err)

	// Test invalid JSON
	err = spout.processMessage([]byte("invalid json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error unmarshaling message")

	// Test with parsing disabled
	spout.isParse = false
	err = spout.processMessage([]byte("raw message"))
	assert.NoError(t, err)
}

func TestSpout_ContextCancellation(t *testing.T) {
	org := createTestOrganization()

	spout, err := NewSpout(org, "event")
	require.NoError(t, err)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Replace the spout's context
	spout.ctx = ctx
	spout.cancel = cancel

	// Start a goroutine to read messages
	done := make(chan struct{})
	go func() {
		_, err := spout.Get()
		assert.Error(t, err)
		assert.Equal(t, "spout stopped", err.Error())
		close(done)
	}()

	// Wait for context cancellation
	<-ctx.Done()
	<-done
}
