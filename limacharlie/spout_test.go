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
			defer got.Shutdown()

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
	defer org.Close()

	// Create a Spout with a real connection
	spout, err := NewSpout(org, "event")
	defer spout.Shutdown()
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
	defer org.Close()

	spout, err := NewSpout(org, "event", WithMaxBuffer(1))
	defer spout.Shutdown()
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
	defer org.Close()

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
	defer org.Close()

	spout, err := NewSpout(org, "event")
	defer spout.Shutdown()
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
	defer org.Close()

	spout, err := NewSpout(org, "event")
	defer spout.Shutdown()
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
	defer org.Close()
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

func TestSpout_SimpleRequest(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a).WithInvestigationID("test-cicd")
	defer org.Close()

	// List all sensors to find an online one
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

	// Create a sensor instance
	sensor := org.GetSensor(targetSID)
	require.NotNil(t, sensor)

	// Make organization interactive
	err = org.MakeInteractive()
	require.NoError(t, err)

	// Send a simple task
	resp, err := sensor.SimpleRequest("os_version", SimpleRequestOptions{
		Timeout: 30 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify response structure
	if m, ok := resp.(map[string]interface{}); ok {
		// Check for expected fields in system info
		require.Contains(t, m, "event")
		require.NotEmpty(t, m["event"])
	} else {
		t.Errorf("unexpected response type: %T", resp)
	}
}

func TestFutureResults(t *testing.T) {
	// Create a FutureResults
	future := NewFutureResults(10)
	defer future.Close()

	// Test adding results
	testData := map[string]interface{}{"test": "data"}
	success := future.addResult(testData)
	assert.True(t, success, "should be able to add result")

	// Test retrieving result
	result, ok := future.Get()
	assert.True(t, ok, "should be able to get result")
	assert.Equal(t, testData, result, "result should match what was added")

	// Test timeout
	_, err := future.GetWithTimeout(100 * time.Millisecond)
	assert.Error(t, err, "should timeout when no data available")
	assert.Contains(t, err.Error(), "timeout", "error should mention timeout")
}

func TestSpout_FutureResults(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	spout, err := NewSpout(org, "event")
	defer spout.Shutdown()
	require.NoError(t, err)

	// Create a FutureResults
	future := NewFutureResults(10)
	trackingID := "test-tracking-id-123"

	// Register the future
	spout.RegisterFutureResults(trackingID, future, 1*time.Hour)

	// Verify it was registered
	spout.futuresMu.RLock()
	_, exists := spout.futures[trackingID]
	spout.futuresMu.RUnlock()
	assert.True(t, exists, "future should be registered")

	// Create a test message with the tracking ID
	testMsg := map[string]interface{}{
		"routing": map[string]interface{}{
			"investigation_id": trackingID,
		},
		"data": "test",
	}
	msgBytes, _ := json.Marshal(testMsg)

	// Process the message
	err = spout.processMessage(msgBytes)
	assert.NoError(t, err, "should process message without error")

	// Verify the message was routed to the future
	result, err := future.GetWithTimeout(1 * time.Second)
	assert.NoError(t, err, "should get result from future")
	if m, ok := result.(map[string]interface{}); ok {
		assert.Equal(t, "test", m["data"], "message should be routed to future")
	}
}

func TestSpout_FutureResultsCleanup(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	spout, err := NewSpout(org, "event")
	defer spout.Shutdown()
	require.NoError(t, err)

	// Create a FutureResults with very short TTL
	future := NewFutureResults(10)
	trackingID := "test-tracking-id-expired"

	// Register the future with short TTL
	spout.RegisterFutureResults(trackingID, future, 100*time.Millisecond)

	// Verify it was registered
	spout.futuresMu.RLock()
	_, exists := spout.futures[trackingID]
	spout.futuresMu.RUnlock()
	assert.True(t, exists, "future should be registered")

	// Wait for it to expire
	time.Sleep(200 * time.Millisecond)

	// Manually trigger cleanup
	spout.futuresMu.Lock()
	now := time.Now()
	for tid, reg := range spout.futures {
		if now.After(reg.expiry) {
			reg.future.Close()
			delete(spout.futures, tid)
		}
	}
	spout.futuresMu.Unlock()

	// Verify it was cleaned up
	spout.futuresMu.RLock()
	_, exists = spout.futures[trackingID]
	spout.futuresMu.RUnlock()
	assert.False(t, exists, "expired future should be cleaned up")
}

func TestSpout_ReconnectOption(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Test with reconnect enabled (default)
	spout1, err := NewSpout(org, "event")
	require.NoError(t, err)
	assert.True(t, spout1.reconnectEnabled, "reconnect should be enabled by default")
	spout1.Shutdown()

	// Test with reconnect disabled
	spout2, err := NewSpout(org, "event", WithReconnect(false))
	require.NoError(t, err)
	assert.False(t, spout2.reconnectEnabled, "reconnect should be disabled when specified")
	spout2.Shutdown()
}

func TestFutureResults_GetNewResponses(t *testing.T) {
	// Create a FutureResults
	future := NewFutureResults(10)
	defer future.Close()

	// Test adding multiple results
	testData1 := map[string]interface{}{"test": "data1"}
	testData2 := map[string]interface{}{"test": "data2"}
	testData3 := map[string]interface{}{"test": "data3"}

	success := future.addResult(testData1)
	assert.True(t, success, "should be able to add result 1")
	success = future.addResult(testData2)
	assert.True(t, success, "should be able to add result 2")
	success = future.addResult(testData3)
	assert.True(t, success, "should be able to add result 3")

	// Test batch retrieval
	results := future.GetNewResponses(1 * time.Second)
	assert.Len(t, results, 3, "should retrieve all 3 results")

	// Verify the results
	assert.Equal(t, testData1, results[0])
	assert.Equal(t, testData2, results[1])
	assert.Equal(t, testData3, results[2])

	// Test that accumulated results are cleared after retrieval
	results = future.GetNewResponses(100 * time.Millisecond)
	assert.Empty(t, results, "should return empty slice after timeout")
}

func TestFutureResults_WasReceived(t *testing.T) {
	// Create a FutureResults
	future := NewFutureResults(10)
	defer future.Close()

	// Initially wasReceived should be false
	assert.False(t, future.WasReceived, "wasReceived should start as false")

	// Add a CLOUD_NOTIFICATION event
	cloudNotification := map[string]interface{}{
		"routing": map[string]interface{}{
			"event_type": "CLOUD_NOTIFICATION",
		},
		"event": map[string]interface{}{},
	}
	success := future.addResult(cloudNotification)
	assert.True(t, success, "should be able to add CLOUD_NOTIFICATION")

	// Verify wasReceived is now true
	assert.True(t, future.WasReceived, "wasReceived should be true after CLOUD_NOTIFICATION")

	// Verify the notification is in the queue (for backward compatibility)
	msg, ok := future.Get()
	assert.True(t, ok, "should get notification from queue")
	assert.Equal(t, cloudNotification, msg)
}

func TestFutureResults_WasReceivedWithRegularEvents(t *testing.T) {
	// Create a FutureResults
	future := NewFutureResults(10)
	defer future.Close()

	// Add regular events (not CLOUD_NOTIFICATION)
	regularEvent := map[string]interface{}{
		"routing": map[string]interface{}{
			"event_type": "SOME_OTHER_EVENT",
		},
		"event": map[string]interface{}{
			"data": "test",
		},
	}
	success := future.addResult(regularEvent)
	assert.True(t, success, "should be able to add regular event")

	// Verify wasReceived is still false
	assert.False(t, future.WasReceived, "wasReceived should remain false for non-CLOUD_NOTIFICATION events")

	// Verify the event is both in queue and accumulated results
	msg, ok := future.Get()
	assert.True(t, ok, "should get event from queue")
	assert.Equal(t, regularEvent, msg)

	results := future.GetNewResponses(1 * time.Second)
	assert.Len(t, results, 1, "should have 1 accumulated result")
	assert.Equal(t, regularEvent, results[0])
}

func TestSpout_ExtraParams(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Test creating a spout with extra params
	extraParams := map[string]interface{}{
		"custom_param1": "value1",
		"custom_param2": 123,
		"custom_param3": true,
	}

	spout, err := NewSpout(org, "event", WithExtraParams(extraParams))
	require.NoError(t, err)
	defer spout.Shutdown()

	// Verify extra params are stored
	assert.Equal(t, extraParams, spout.extraParams, "extra params should be stored")

	// Note: We can't easily test that the extra params are actually sent to the server
	// without mocking the WebSocket connection, but the implementation will include them
	// in the header via connectWebSocket method
}

func TestSpout_ExtraParamsWithStart(t *testing.T) {
	a := assert.New(t)
	org := getTestOrgFromEnv(a)
	defer org.Close()

	// Create spout with extra params
	extraParams := map[string]interface{}{
		"test_param": "test_value",
	}

	spout, err := NewSpout(org, "event", WithExtraParams(extraParams))
	require.NoError(t, err)
	defer spout.Shutdown()

	// Start the spout - this will use connectWebSocket which handles extra params
	err = spout.Start()
	require.NoError(t, err)

	// If we get here without error, the connection was successful
	// which means the server accepted our request with extra params
	assert.NotNil(t, spout.conn, "connection should be established")
}
