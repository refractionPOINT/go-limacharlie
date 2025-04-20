package limacharlie

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSpout(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

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
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// Create a Spout with a real connection
	spout, err := NewSpout(org, "event")
	require.NoError(t, err)

	// Start the spout
	err = spout.Start()
	require.NoError(t, err)

	// Create a context with timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a goroutine to read messages
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := spout.Get()
				if err != nil {
					if err.Error() == "spout stopped" {
						return
					}
					t.Logf("error getting message: %v", err)
					continue
				}
				t.Logf("received message: %v", msg)
			}
		}
	}()

	// Wait a bit to receive some messages
	time.Sleep(2 * time.Second)

	// Shutdown
	spout.Shutdown()

	// Wait for the reader to finish
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("test timed out")
	}

	// Verify shutdown
	_, err = spout.Get()
	if err != nil {
		assert.Equal(t, "spout stopped", err.Error())
	}
}

func TestSpout_DroppedMessages(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	spout, err := NewSpout(org, "event", WithMaxBuffer(1))
	require.NoError(t, err)

	// Create a test message
	msg := map[string]interface{}{"test": "message"}

	// Fill the buffer
	spout.queue <- msg

	// Try to add another message (should be dropped)
	err = spout.processMessage([]byte(`{"test":"message2"}`))
	assert.Error(t, err)
	assert.Equal(t, "queue full after timeout", err.Error())

	// Verify dropped count
	assert.Equal(t, int64(1), spout.GetDropped())

	// Reset counter
	spout.ResetDroppedCounter()
	assert.Equal(t, int64(0), spout.GetDropped())
}

func TestSpout_TraceMessages(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

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
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

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
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

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

func TestSpout_SpecificSensor(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)

	// List all sensors
	sensors, err := org.ListSensors()
	require.NoError(t, err)
	require.NotEmpty(t, sensors, "no sensors found")

	// Find an online Linux sensor
	var targetSID string
	for sid, sensor := range sensors {
		if sensor.Platform == Platforms.Linux {
			isOnline, err := sensor.IsOnline()
			if err == nil && isOnline {
				targetSID = sid
				break
			}
		}
	}
	require.NotEmpty(t, targetSID, "no online Linux sensor found")

	t.Logf("targetSID: %v", targetSID)

	// Create a Spout specifically for this sensor
	spout, err := NewSpout(org, "event", WithSensorID(targetSID))
	require.NoError(t, err)

	// Start the spout
	err = spout.Start()
	require.NoError(t, err)

	// Create a context with timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start a goroutine to read messages
	done := make(chan struct{})
	eventsReceived := 0
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := spout.Get()
				if err != nil {
					if err.Error() == "spout stopped" {
						return
					}
					t.Logf("error getting message: %v", err)
					continue
				}
				if d, ok := msg.(map[string]interface{}); ok {
					if len(d) == 0 {
						t.Errorf("received empty message: %v", msg)
					}
				} else {
					t.Errorf("received non-map message: %v", msg)
				}
				eventsReceived++
				if eventsReceived >= 2 {
					return
				}
			}
		}
	}()

	// Wait for either 2 events or timeout
	select {
	case <-done:
		assert.GreaterOrEqual(t, eventsReceived, 2, "did not receive at least 2 events")
	case <-ctx.Done():
		t.Fatal("test timed out waiting for events")
	}

	// Shutdown
	spout.Shutdown()
}
