package notifications

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"shared/messagebus"
	"shared/models"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNats(t *testing.T, port int) (*nats.Conn, *server.Server) {
	opts := natsserver.DefaultTestOptions
	opts.Port = port
	server := natsserver.RunServer(&opts)

	nc, err := nats.Connect("nats://127.0.0.1:" + strconv.Itoa(port))
	require.NoError(t, err, "Should connect to NATS")
	return nc, server
}

func setupWs(hub *Hub) *httptest.Server {
	handler := NewHandler(hub, slog.New(slog.DiscardHandler))
	wsServer := httptest.NewServer(http.HandlerFunc(handler.HandleWebSocket))
	return wsServer
}

func setupIntegration(t *testing.T) (*messagebus.MessageBus, string, func()) {
	nc, server := setupNats(t, 8400)

	hub := NewHub(WithHubLogger(slog.New(slog.DiscardHandler)))
	wsServer := setupWs(hub)
	mb := messagebus.New(nc, nil)

	svc := NewNotificationService(
		hub,
		mb,
		WithLogger(slog.New(slog.DiscardHandler)),
	)

	svc.Start(context.Background())

	shutdown := func() {
		svc.Stop()
		server.Shutdown()
		nc.Close()
		wsServer.Close()
	}

	wsURL := "ws" + strings.TrimPrefix(wsServer.URL, "http")
	return mb, wsURL, shutdown
}

func TestNotificationService_JobUpdateBroadcast_Integration(t *testing.T) {
	mb, wsURL, shutdown := setupIntegration(t)
	defer shutdown()

	time.Sleep(200 * time.Millisecond)

	var clients []*websocket.Conn

	for i := 0; i < 3; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err, "Should connect WebSocket client %d", i+1)
		clients = append(clients, conn)
	}

	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Publish job update through NATS
	jobMsg := messagebus.JobUpdateMessage{
		Type:   messagebus.JobUpdateMessageType,
		JobID:  "integration-job-123",
		Status: "completed",
		Result: &models.AnalyzeResult{
			PageTitle:         "Integration Test Page",
			HtmlVersion:       "HTML5",
			InternalLinkCount: 10,
			ExternalLinkCount: 5,
			HasLoginForm:      true,
		},
	}

	err := mb.PublishJobUpdate(context.Background(), jobMsg)
	require.NoError(t, err, "Should publish job update")

	time.Sleep(300 * time.Millisecond)

	// Verify all clients received the job update
	for i, client := range clients {
		client.SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msgData, err := client.ReadMessage()
		require.NoError(t, err, "Client %d should receive job update message", i+1)

		var received messagebus.JobUpdateMessage
		err = json.Unmarshal(msgData, &received)
		require.NoError(t, err, "Should unmarshal job update for client %d", i+1)

		assert.Equal(t, jobMsg.Type, received.Type, "Message type should match for client %d", i+1)
		assert.Equal(t, jobMsg.JobID, received.JobID, "Job ID should match for client %d", i+1)
		assert.Equal(t, jobMsg.Status, received.Status, "Status should match for client %d", i+1)
		assert.NotNil(t, received.Result, "Result should be present for client %d", i+1)
		assert.Equal(t, jobMsg.Result.PageTitle, received.Result.PageTitle, "Page title should match for client %d", i+1)
	}
}

func TestNotificationService_GroupSubscription_Integration(t *testing.T) {
	mb, wsURL, shutdown := setupIntegration(t)
	defer shutdown()

	time.Sleep(200 * time.Millisecond)

	var clients []*websocket.Conn

	for i := 0; i < 3; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err, "Should connect WebSocket client %d", i+1)
		clients = append(clients, conn)
	}

	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Subscribe first 2 clients to the target group
	subMsg := SubscriptionMessage{Action: "subscribe", Group: "target-job-456"}
	msgData, err := json.Marshal(subMsg)
	require.NoError(t, err, "Should marshal subscription message")

	for i := 0; i < 2; i++ {
		err = clients[i].WriteMessage(websocket.TextMessage, msgData)
		require.NoError(t, err, "Should send subscription for client %d", i+1)
	}

	// Client 3 remains unsubscribed
	time.Sleep(100 * time.Millisecond)

	// Publish task status update for the target group
	taskMsg := messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    "target-job-456",
		TaskType: "analyzing",
		Status:   "running",
	}

	err = mb.PublishTaskStatusUpdate(context.Background(), taskMsg)
	require.NoError(t, err, "Should publish task status update")

	time.Sleep(300 * time.Millisecond)

	// Verify first 2 clients received the message (subscribed)
	for i := 0; i < 2; i++ {
		clients[i].SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msgData, err := clients[i].ReadMessage()
		require.NoError(t, err, "Subscribed client %d should receive task update", i+1)

		var received messagebus.TaskStatusUpdateMessage
		err = json.Unmarshal(msgData, &received)
		require.NoError(t, err, "Should unmarshal task update for client %d", i+1)

		assert.Equal(t, taskMsg.Type, received.Type, "Message type should match for client %d", i+1)
		assert.Equal(t, taskMsg.JobID, received.JobID, "Job ID should match for client %d", i+1)
		assert.Equal(t, taskMsg.TaskType, received.TaskType, "Task type should match for client %d", i+1)
		assert.Equal(t, taskMsg.Status, received.Status, "Status should match for client %d", i+1)

	}

	// Third client should NOT receive the message (unsubscribed)
	clients[2].SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = clients[2].ReadMessage()
	assert.Error(t, err, "Unsubscribed client should not receive group-specific message")
}

func TestNotificationService_ConcurrentClients_Integration(t *testing.T) {
	mb, wsURL, shutdown := setupIntegration(t)
	defer shutdown()

	time.Sleep(200 * time.Millisecond)

	// Connect 5 concurrent clients
	const clientCount = 5
	var clients []*websocket.Conn

	for i := 0; i < clientCount; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err, "Should connect WebSocket client %d", i+1)
		clients = append(clients, conn)
	}

	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	time.Sleep(200 * time.Millisecond)

	// Subscribe half the clients to a specific group
	subMsg := SubscriptionMessage{Action: "subscribe", Group: "concurrent-test-job"}
	msgData, err := json.Marshal(subMsg)
	require.NoError(t, err, "Should marshal subscription message")

	subscribedCount := clientCount / 2
	for i := 0; i < subscribedCount; i++ {
		err = clients[i].WriteMessage(websocket.TextMessage, msgData)
		require.NoError(t, err, "Should subscribe client %d", i+1)
	}

	time.Sleep(100 * time.Millisecond)

	// Publish a global job update (should reach all clients)
	globalMsg := messagebus.JobUpdateMessage{
		Type:   messagebus.JobUpdateMessageType,
		JobID:  "global-concurrent-job",
		Status: "processing",
		Result: &models.AnalyzeResult{
			PageTitle:   "Concurrent Test",
			HtmlVersion: "HTML5",
		},
	}

	err = mb.PublishJobUpdate(context.Background(), globalMsg)
	require.NoError(t, err, "Should publish global job update")

	time.Sleep(300 * time.Millisecond)

	// Verify all clients received the global message
	globalReceivedCount := 0
	for _, client := range clients {
		client.SetReadDeadline(time.Now().Add(time.Second))
		_, msgData, err := client.ReadMessage()
		if err == nil {
			var received messagebus.JobUpdateMessage
			if json.Unmarshal(msgData, &received) == nil && received.JobID == "global-concurrent-job" {
				globalReceivedCount++
			}
		}
	}

	assert.Equal(t, clientCount, globalReceivedCount, "All clients should receive global broadcast")

	// Publish a group-specific task update (should reach only subscribed clients)
	groupMsg := messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    "concurrent-test-job",
		TaskType: "processing",
		Status:   "running",
	}

	err = mb.PublishTaskStatusUpdate(context.Background(), groupMsg)
	require.NoError(t, err, "Should publish group task update")

	time.Sleep(300 * time.Millisecond)

	// Verify only subscribed clients received the group message
	groupReceivedCount := 0
	for i, client := range clients {
		client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msgData, err := client.ReadMessage()
		if err == nil {
			var received messagebus.TaskStatusUpdateMessage
			if json.Unmarshal(msgData, &received) == nil && received.JobID == "concurrent-test-job" {
				groupReceivedCount++
				assert.True(t, i < subscribedCount, "Only subscribed clients should receive group message")
			}
		}
	}

	assert.Equal(t, subscribedCount, groupReceivedCount, "Only subscribed clients should receive group message")
}

func TestNotificationService_SubTaskUpdate_Integration(t *testing.T) {
	mb, wsURL, shutdown := setupIntegration(t)
	defer shutdown()

	time.Sleep(200 * time.Millisecond)

	var clients []*websocket.Conn

	// Create 4 clients
	for i := 0; i < 4; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err, "Should connect WebSocket client %d", i+1)
		clients = append(clients, conn)
	}

	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Subscribe clients to different groups
	// Client 1 & 2: subscribe to "subtask-job-789"
	// Client 3: subscribe to "other-job"
	// Client 4: no subscription

	subMsg1 := SubscriptionMessage{Action: "subscribe", Group: "subtask-job-789"}
	msgData1, err := json.Marshal(subMsg1)
	require.NoError(t, err, "Should marshal subscription message 1")

	for i := 0; i < 2; i++ {
		err = clients[i].WriteMessage(websocket.TextMessage, msgData1)
		require.NoError(t, err, "Should subscribe client %d to subtask-job-789", i+1)
	}

	subMsg2 := SubscriptionMessage{Action: "subscribe", Group: "other-job"}
	msgData2, err := json.Marshal(subMsg2)
	require.NoError(t, err, "Should marshal subscription message 2")

	err = clients[2].WriteMessage(websocket.TextMessage, msgData2)
	require.NoError(t, err, "Should subscribe client 3 to other-job")

	time.Sleep(100 * time.Millisecond)

	// Publish subtask update for the target group
	subTaskMsg := messagebus.SubTaskUpdateMessage{
		Type:     messagebus.SubTaskUpdateMessageType,
		JobID:    "subtask-job-789",
		TaskType: "link_validation",
		Key:      "link-integration-test",
		SubTask: models.SubTask{
			Type:        models.SubTaskTypeValidatingLink,
			Status:      models.TaskStatusCompleted,
			URL:         "https://integration-test.example.com",
			Description: "Integration test link validation complete",
		},
	}

	err = mb.PublishSubTaskUpdate(context.Background(), subTaskMsg)
	require.NoError(t, err, "Should publish subtask update")

	time.Sleep(300 * time.Millisecond)

	// Verify first 2 clients received the message (subscribed to subtask-job-789)
	for i := 0; i < 2; i++ {
		clients[i].SetReadDeadline(time.Now().Add(3 * time.Second))
		_, msgData, err := clients[i].ReadMessage()
		require.NoError(t, err, "Client %d subscribed to subtask-job-789 should receive subtask update", i+1)

		var received messagebus.SubTaskUpdateMessage
		err = json.Unmarshal(msgData, &received)
		require.NoError(t, err, "Should unmarshal subtask update for client %d", i+1)

		assert.Equal(t, subTaskMsg.Type, received.Type, "Message type should match for client %d", i+1)
		assert.Equal(t, subTaskMsg.JobID, received.JobID, "Job ID should match for client %d", i+1)
		assert.Equal(t, subTaskMsg.Key, received.Key, "SubTask key should match for client %d", i+1)
		assert.Equal(t, subTaskMsg.SubTask.Status, received.SubTask.Status, "SubTask status should match for client %d", i+1)
		assert.Equal(t, subTaskMsg.SubTask.URL, received.SubTask.URL, "SubTask URL should match for client %d", i+1)
		assert.Equal(t, subTaskMsg.SubTask.Description, received.SubTask.Description, "SubTask description should match for client %d", i+1)
	}

	// Clients 3 & 4 should NOT receive the message
	for i := 2; i < 4; i++ {
		clients[i].SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, _, err := clients[i].ReadMessage()
		assert.Error(t, err, "Client %d should not receive message for different/no group", i+1)
	}
}

func TestNotificationService_SubscriptionLifecycle_Integration(t *testing.T) {
	mb, wsURL, shutdown := setupIntegration(t)
	defer shutdown()

	time.Sleep(200 * time.Millisecond)

	// Create client and subscribe
	client1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Should connect WebSocket client 1")

	subMsg := SubscriptionMessage{Action: "subscribe", Group: "lifecycle-job-456"}
	msgData, err := json.Marshal(subMsg)
	require.NoError(t, err, "Should marshal subscription message")

	err = client1.WriteMessage(websocket.TextMessage, msgData)
	require.NoError(t, err, "Should subscribe client1")

	time.Sleep(200 * time.Millisecond)

	// Publish first message - client should receive
	taskMsg1 := messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    "lifecycle-job-456",
		TaskType: "phase1_test",
		Status:   "running",
	}

	err = mb.PublishTaskStatusUpdate(context.Background(), taskMsg1)
	require.NoError(t, err, "Should publish first task update")

	time.Sleep(300 * time.Millisecond)

	// Verify client received the message
	client1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgData1, err := client1.ReadMessage()
	require.NoError(t, err, "Client should receive first task update")

	var received1 messagebus.TaskStatusUpdateMessage
	err = json.Unmarshal(msgData1, &received1)
	require.NoError(t, err, "Should unmarshal first task update")

	assert.Equal(t, taskMsg1.JobID, received1.JobID, "First message JobID should match")
	assert.Equal(t, taskMsg1.TaskType, received1.TaskType, "First message TaskType should match")

	// Unsubscribe and close connection
	unsubMsg := SubscriptionMessage{Action: "unsubscribe", Group: "lifecycle-job-456"}
	unsubData, err := json.Marshal(unsubMsg)
	require.NoError(t, err, "Should marshal unsubscription message")

	err = client1.WriteMessage(websocket.TextMessage, unsubData)
	require.NoError(t, err, "Should unsubscribe client1")

	time.Sleep(200 * time.Millisecond)
	client1.Close()

	// Publish second message - no one should receive
	taskMsg2 := messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    "lifecycle-job-456",
		TaskType: "phase2_test",
		Status:   "completed",
	}

	err = mb.PublishTaskStatusUpdate(context.Background(), taskMsg2)
	require.NoError(t, err, "Should publish second task update")

	time.Sleep(300 * time.Millisecond)

	// Create fresh connection and re-subscribe
	client2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Should connect fresh WebSocket client")
	defer client2.Close()

	resubMsg := SubscriptionMessage{Action: "subscribe", Group: "lifecycle-job-456"}
	resubData, err := json.Marshal(resubMsg)
	require.NoError(t, err, "Should marshal re-subscription message")

	err = client2.WriteMessage(websocket.TextMessage, resubData)
	require.NoError(t, err, "Should re-subscribe with fresh client")

	time.Sleep(200 * time.Millisecond)

	// Publish third message - fresh client should receive
	taskMsg3 := messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    "lifecycle-job-456",
		TaskType: "phase3_test",
		Status:   "restarted",
	}

	err = mb.PublishTaskStatusUpdate(context.Background(), taskMsg3)
	require.NoError(t, err, "Should publish third task update")

	time.Sleep(300 * time.Millisecond)

	// Verify fresh client received the third message
	client2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgData3, err := client2.ReadMessage()
	require.NoError(t, err, "Fresh client should receive third task update")

	var received3 messagebus.TaskStatusUpdateMessage
	err = json.Unmarshal(msgData3, &received3)
	require.NoError(t, err, "Should unmarshal third task update")

	assert.Equal(t, taskMsg3.JobID, received3.JobID, "Third message JobID should match")
	assert.Equal(t, taskMsg3.TaskType, received3.TaskType, "Third message TaskType should match")
	assert.Equal(t, "restarted", received3.Status, "Third message Status should match")
}

func TestNotificationService_UnsubscribeGroup_Integration(t *testing.T) {
	mb, wsURL, shutdown := setupIntegration(t)
	defer shutdown()

	time.Sleep(200 * time.Millisecond)

	// Create 1 client
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "Should connect WebSocket client")
	defer client.Close()

	time.Sleep(100 * time.Millisecond)

	// Subscribe to target group
	subMsg := SubscriptionMessage{Action: "subscribe", Group: "unsubscribe-test-job"}
	msgData, err := json.Marshal(subMsg)
	require.NoError(t, err, "Should marshal subscription message")

	err = client.WriteMessage(websocket.TextMessage, msgData)
	require.NoError(t, err, "Should subscribe client")

	time.Sleep(200 * time.Millisecond)

	// Publish message - client should receive
	taskMsg := messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    "unsubscribe-test-job",
		TaskType: "test_task",
		Status:   "running",
	}

	err = mb.PublishTaskStatusUpdate(context.Background(), taskMsg)
	require.NoError(t, err, "Should publish task update")

	time.Sleep(300 * time.Millisecond)

	// Verify client received the message
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, receivedData, err := client.ReadMessage()
	require.NoError(t, err, "Client should receive task update while subscribed")

	var received messagebus.TaskStatusUpdateMessage
	err = json.Unmarshal(receivedData, &received)
	require.NoError(t, err, "Should unmarshal task update")

	assert.Equal(t, taskMsg.JobID, received.JobID, "Message JobID should match")
	assert.Equal(t, taskMsg.TaskType, received.TaskType, "Message TaskType should match")

	// Unsubscribe from the group
	unsubMsg := SubscriptionMessage{Action: "unsubscribe", Group: "unsubscribe-test-job"}
	unsubData, err := json.Marshal(unsubMsg)
	require.NoError(t, err, "Should marshal unsubscription message")

	err = client.WriteMessage(websocket.TextMessage, unsubData)
	require.NoError(t, err, "Should unsubscribe client")

	time.Sleep(200 * time.Millisecond)

	// Publish another message - client should NOT receive
	taskMsg2 := messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    "unsubscribe-test-job",
		TaskType: "test_task_2",
		Status:   "completed",
	}

	err = mb.PublishTaskStatusUpdate(context.Background(), taskMsg2)
	require.NoError(t, err, "Should publish second task update")

	time.Sleep(300 * time.Millisecond)

	// Verify client did NOT receive the second message
	client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = client.ReadMessage()
	assert.Error(t, err, "Client should not receive message after unsubscribing")
}
