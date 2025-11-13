package clients

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/manishagolane/client-nest/data"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

type NATSClient struct {
	nc      *nats.Conn
	js      jetstream.JetStream
	KV      jetstream.KeyValue
	logger  *zap.Logger
	pq      ReminderPriorityQueue
	pqMutex sync.Mutex
}

var newReminderChan = make(chan struct{}, 1) // Channel to notify of new reminders

// Priority Queue Implementation
type ReminderPriorityQueue []*data.Reminder

func (pq ReminderPriorityQueue) Len() int { return len(pq) }
func (pq ReminderPriorityQueue) Less(i, j int) bool {
	return pq[i].RemindTime.Before(pq[j].RemindTime)
}
func (pq ReminderPriorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }
func (pq *ReminderPriorityQueue) Push(x interface{}) {
	r := x.(*data.Reminder)
	log.Printf("Pushing to heap: %+v", r)
	*pq = append(*pq, r)

	// Notify the processing loop
	select {
	case newReminderChan <- struct{}{}:
	default: // Non-blocking: Ensures it doesn't get stuck if the channel is already full
	}
}
func (pq *ReminderPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	log.Printf("Popped from heap: %+v", item)
	return item
}

// Global Singleton Variable
var (
	instance *NATSClient
	once     sync.Once
)

// NewNATSClient (Ensures Only One Instance)
func NewNATSClient(logger *zap.Logger) (*NATSClient, error) {
	var err error
	once.Do(func() {
		instance = &NATSClient{logger: logger}
		instance.nc, err = nats.Connect("nats://localhost:4222",
			nats.ReconnectWait(2*time.Second),
			nats.MaxReconnects(-1),
			nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
				logger.Fatal("disconnected from NATS:", zap.Error(err))
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				logger.Info("reconnected to NATS:", zap.String("ConnectedUrl", nc.ConnectedUrl()))
			}),
		)
		if err != nil {
			logger.Fatal("failed to connect to NATS:", zap.Error(err))
		}

		instance.js, err = jetstream.New(instance.nc)
		if err != nil {
			instance.nc.Close()
			logger.Fatal("failed to initialize JetStream:", zap.Error(err))
		}

		instance.KV, err = instance.js.KeyValue(context.Background(), "ticket_reminders")
		if err != nil {
			instance.KV, err = instance.js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
				Bucket: "ticket_reminders",
			})
			if err != nil {
				logger.Fatal("failed to create KV store:", zap.Error(err))
			}
		}

		instance.createOrUpdateStream()

		// Initialize the priority queue
		instance.pq = ReminderPriorityQueue{}
		heap.Init(&instance.pq)

		// Load existing reminders
		instance.loadExistingReminders()

		// Start the processing loop
		go instance.processReminders()

		// Watch for new reminders in KV Store
		go instance.watchKVUpdates()
		logger.Info("Connected to NATS JetStream")
	})

	if err != nil {
		return nil, err
	}
	return instance, nil
}

// Fetch all stored reminders on startup
func (n *NATSClient) loadExistingReminders() {
	n.logger.Info("Loading existing reminders")
	ctx := context.Background()
	keys, err := n.KV.Keys(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "no keys found") {
			n.logger.Info("No reminders found in KV store, skipping load")
		} else {
			n.logger.Error("Failed to fetch KV keys:", zap.Error(err))
		}
		return
	}

	for _, key := range keys {
		n.logger.Info("Processing key", zap.String("key", key))
		entry, err := n.KV.Get(ctx, key)
		if err == nil {
			var reminder data.Reminder
			err := json.Unmarshal(entry.Value(), &reminder)
			if err != nil {
				n.logger.Error("Error unmarshaling reminder", zap.String("key", key), zap.Error(err))
				continue
			}
			n.logger.Info("Existing reminder:", zap.Any("reminder", reminder))

			if reminder.Status == "pending" {
				n.pqMutex.Lock()
				heap.Push(&n.pq, &reminder)
				n.pqMutex.Unlock()
			}
		}
	}
}

// Watch for KV store updates
func (n *NATSClient) watchKVUpdates() {
	ctx := context.Background()
	watcher, err := n.KV.WatchAll(ctx)
	if err != nil {
		n.logger.Error("Failed to watch KV Store:", zap.Error(err))
		return
	}
	defer watcher.Stop()

	// Process new updates
	for update := range watcher.Updates() {
		if update == nil {
			continue
		}

		key := update.Key()

		// Remove any existing instance of this reminder in heap
		n.pqMutex.Lock()
		for i, item := range n.pq {
			if item.Key == key {
				heap.Remove(&n.pq, i)
				break
			}
		}
		n.pqMutex.Unlock()

		var reminder data.Reminder
		rawValue := update.Value()

		if len(rawValue) == 0 {
			n.logger.Error("Received empty reminder data", zap.String("key", key))
			continue
		}
		n.logger.Info("Raw reminder data received", zap.String("key", key), zap.ByteString("value", rawValue))

		if err := json.Unmarshal(update.Value(), &reminder); err != nil {
			n.logger.Error("failed to parse reminder data", zap.String("key", key), zap.Error(err))
			continue
		}

		if reminder.Status == "pending" {
			n.pqMutex.Lock()
			heap.Push(&n.pq, &reminder)
			n.pqMutex.Unlock()
		}
	}
}

// Process reminders continuously
func (n *NATSClient) processReminders() {
	for {
		n.pqMutex.Lock()
		if len(n.pq) == 0 {
			n.pqMutex.Unlock()
			select {
			case <-newReminderChan: // Wait for a new reminder signal
			case <-time.After(1 * time.Second): // Keep checking periodically
			}
			continue
		}

		reminder := n.pq[0] // Peek at the earliest reminder
		nowUTC := time.Now().UTC()
		n.logger.Info("Current Time Debug",
			zap.String("now_utc", nowUTC.Format(time.RFC3339)),
			zap.String("system_now", time.Now().Format(time.RFC3339)), // Check system timezone
		)
		// Convert the times for logging purposes (IST)
		loc, _ := time.LoadLocation("Asia/Kolkata")
		scheduledTimeIST := reminder.RemindTime.In(loc)
		currentTimeIST := time.Now().In(loc) // Ensure correct conversion

		// timeUntilReminder := time.Until(reminder.RemindTime)
		timeUntilReminder := reminder.RemindTime.Sub(nowUTC)
		n.logger.Info("Processing Reminder",
			zap.String("key", reminder.Key),
			zap.Time("stored_utc", reminder.RemindTime),
			zap.String("parsed_scheduled_utc", reminder.RemindTime.Format(time.RFC3339)),
			zap.String("scheduledTime_IST", scheduledTimeIST.Format("2006-01-02 15:04:05 MST")),
			zap.String("current_time_IST", currentTimeIST.Format("2006-01-02 15:04:05 MST")),
			zap.String("current_time_utc", nowUTC.Format(time.RFC3339)),
			zap.Duration("computed_delay", timeUntilReminder),
		)
		n.pqMutex.Unlock()

		if timeUntilReminder <= 0 {
			// Reminder time has arrived, process it
			n.pqMutex.Lock()
			heap.Pop(&n.pq) // Remove from queue
			n.pqMutex.Unlock()
			n.logger.Info("Executing Reminder", zap.Any("reminder", reminder))
			// _ = DeleteReminder(reminder.Key)
			n.markReminderCompleted(reminder.Key)
			continue
		}
		select {
		case <-time.After(timeUntilReminder): // Wait until the reminder's time
		case <-newReminderChan: // Interrupt if a new reminder is added
		}

	}
}

// Create or Update Stream
func (n *NATSClient) createOrUpdateStream() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := n.js.Stream(ctx, "CRM_TICKETS")
	if err == nil {
		n.logger.Info("Stream CRM_TICKETS already exists")
		return
	}

	_, err = n.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:      "CRM_TICKETS",
		Subjects:  []string{"crm.tickets.*.*"},
		Storage:   jetstream.FileStorage,  // Messages stored on disk
		Retention: jetstream.LimitsPolicy, // Retained until limits are hit
	})
	if err != nil {
		n.logger.Fatal("failed to create stream:", zap.Error(err))
	}

	// Create DLQ Stream
	_, err = n.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:      "CRM_DLQ",
		Subjects:  []string{"crm.dlq.>"},
		Storage:   jetstream.FileStorage,
		Retention: jetstream.LimitsPolicy,
	})
	if err != nil {
		n.logger.Fatal("failed to create DLQ stream:", zap.Error(err))
	}
	n.logger.Info("Created CRM_TICKETS stream")
}

func (n *NATSClient) QueueSubscribe(streamName, subject, consumerName string, handler func(*nats.Msg)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := n.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Durable:        consumerName,
		AckPolicy:      jetstream.AckExplicitPolicy,
		FilterSubjects: []string{subject},
		MaxDeliver:     5,
		AckWait:        30 * time.Second,
	})
	if err != nil {
		n.logger.Error("failed to create or update consumer:", zap.Error(err))
		return err
	}

	sub, err := n.nc.QueueSubscribe(subject, consumerName, func(msg *nats.Msg) {
		handler(msg)
		msg.Ack()
	})
	if err != nil {
		n.logger.Error("failed to subscribe::", zap.Error(err))
		return err
	}

	n.logger.Info("Queue Subscription established", zap.String("subject", subject), zap.String("consumer", consumerName))
	return sub.SetPendingLimits(-1, -1) // Allow unlimited pending messages
}

// Publish Event to JetStream
func (n *NATSClient) PublishEvent(subject string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := n.js.Publish(ctx, subject, data)
	if err != nil {
		n.logger.Fatal("failed to publish event:", zap.Error(err))
		return err
	}
	n.logger.Info("Published event::", zap.String("Subject:", subject))
	return nil
}

func (n *NATSClient) StoreReminderInKV(ctx context.Context, key string, timestamp time.Time, eventData []byte) error {
	reminder := data.Reminder{
		Key:        key,
		EventData:  eventData,
		RemindTime: timestamp.UTC(), // Use time.Time directly
		Status:     "pending",
	}
	reminderJSON, err := json.Marshal(reminder)
	if err != nil {
		n.logger.Error("Failed to marshal reminder", zap.Error(err))
		return err
	}

	// Use CAS for atomic update
	entry, err := n.KV.Get(ctx, key)
	if err != nil { // Key does not exist, create new
		_, err = n.KV.Create(ctx, key, reminderJSON)
	} else { // Key exists, update it safely
		_, err = n.KV.Update(ctx, key, reminderJSON, entry.Revision())
	}
	if err != nil {
		n.logger.Error("failed to store reminder", zap.String("key", key), zap.Error(err))
		return err
	}

	// _, err := n.KV.Put(ctx, key, reminderJSON)
	// if err != nil {
	//  n.logger.Error("Failed to store reminder", zap.String("key", key), zap.Error(err))
	//  return err
	// }
	n.logger.Info("Reminder stored", zap.String("key", key), zap.String("scheduled_time", timestamp.UTC().Format(time.RFC3339)))
	return nil
}

// **Trigger Reminder Event**
func (n *NATSClient) triggerReminder(key string, eventData json.RawMessage) {
	n.logger.Info("Executing Reminder", zap.String("Key", key), zap.ByteString("Data", eventData))
	ticketID := strings.TrimPrefix(key, "reminder_")
	subject := fmt.Sprintf("crm.tickets.reminder.%s", ticketID)

	err := n.PublishEvent(subject, eventData)
	if err != nil {
		n.logger.Error("Failed to publish reminder event", zap.String("ticket_id", ticketID), zap.Error(err))
	} else {
		n.logger.Info("Reminder triggered successfully", zap.String("ticket_id", ticketID))
		// n.KV.Delete(context.Background(), key)
	}
}

// Atomic Processing with CAS Update**
func (n *NATSClient) markReminderCompleted(key string) {
	ctx := context.Background()
	entry, err := n.KV.Get(ctx, key)
	if err != nil {
		n.logger.Error("Reminder not found in KV store")
		return
	}

	var reminder data.Reminder
	err = json.Unmarshal(entry.Value(), &reminder)
	if err != nil {
		n.logger.Error("Failed to unmarshal stored reminder", zap.Error(err))
	}

	reminder.Status = "completed"
	reminder.Revision = entry.Revision()

	updatedData, err := json.Marshal(reminder)
	if err != nil {
		n.logger.Error("failed to update reminder status:", zap.Error(err))
		return
	}
	rev, err := n.KV.Put(ctx, key, updatedData)
	if err != nil {
		n.logger.Error("Failed to update KV store", zap.String("key", key), zap.Error(err))
		return
	}
	n.logger.Info("Updated reminder in KV store", zap.String("key", key), zap.Uint64("new_revision", rev))

	n.triggerReminder(key, reminder.EventData)
}

func (n *NATSClient) CancelReminder(ctx context.Context, key string, eventData []byte, reminderEntry data.Reminder) error {
	n.pqMutex.Lock()
	defer n.pqMutex.Unlock()

	for i, item := range n.pq {
		if item.Key == key {
			heap.Remove(&n.pq, i)
			n.logger.Info("Removed reminder from queue", zap.String("key", key))
			break
		}
	}

	reminderEntry.Status = "canceled"
	updatedData, err := json.Marshal(reminderEntry)
	if err != nil {
		n.logger.Error("failed to update reminder status:", zap.Error(err))
		return nil
	}
	rev, err := n.KV.Put(ctx, key, updatedData)
	if err != nil {
		n.logger.Error("failed to update KV store", zap.String("key", key), zap.Error(err))
		return nil
	}
	n.logger.Info("Updated reminder in KV store", zap.String("key", key), zap.Uint64("new_revision", rev))

	return nil
}

func (n *NATSClient) SnoozeReminder(ctx context.Context, key string, snoozeDuration time.Duration) error {
	n.pqMutex.Lock()
	defer n.pqMutex.Unlock()

	entry, err := n.KV.Get(ctx, key)
	if err != nil {
		n.logger.Error("failed to fetch reminder from KV store", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("reminder not found")
	}

	var reminder data.Reminder
	if err := json.Unmarshal(entry.Value(), &reminder); err != nil {
		n.logger.Error("failed to unmarshal reminder data", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("invalid reminder data")
	}

	// Update the remind time
	reminder.RemindTime = reminder.RemindTime.Add(snoozeDuration)
	n.logger.Info("Snoozing reminder", zap.String("key", key), zap.Time("new_remind_time", reminder.RemindTime))

	// Serialize the updated reminder
	updatedData, err := json.Marshal(reminder)
	if err != nil {
		n.logger.Error("failed to marshal updated reminder", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("failed to update reminder")
	}

	// Save updated reminder back to the KV store
	_, err = n.KV.Put(ctx, key, updatedData)
	if err != nil {
		n.logger.Error("failed to update reminder in KV store", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("failed to save updated reminder")
	}

	// Remove any existing instance of this reminder in the priority queue
	for i, item := range n.pq {
		if item.Key == key {
			heap.Remove(&n.pq, i)
			break
		}
	}

	// Push the updated reminder back into the priority queue
	heap.Push(&n.pq, &reminder)
	n.logger.Info("Updated reminder snoozed and reinserted into queue", zap.String("key", key))

	return nil
}

// Close Connection Gracefully
func (n *NATSClient) Close() {
	if n.nc != nil {
		n.nc.Drain()
		n.logger.Info("NATS connection closed gracefully.")
	}
}
