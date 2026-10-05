package gateway

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"memdoor/gateway/logs"
	"memdoor/gateway/websocket"
	"memdoor/pkg/channel"
	"memdoor/pkg/domain"
	"memdoor/pkg/message"
	"memdoor/pkg/reaction"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"

	"github.com/google/uuid"
)

func TestMain(m *testing.M) {
	// Every gateway asks the broker for a brain by default (ADR-0012, the
	// TUI and the app alike). A test server must not: the first run after
	// that default leased real brains from memdoor.ai for eight minutes.
	os.Setenv("MEMDOOR_BRAIN_CLASS", "local")
	// Initialize the global logger for handler tests (writes to temp dir)
	tmpDir, err := os.MkdirTemp("", "gateway-test-logs-*")
	if err != nil {
		panic("failed to create temp log dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	if err := logs.InitGlobalLogger(tmpDir, false); err != nil {
		panic("failed to init global logger: " + err.Error())
	}

	os.Exit(m.Run())
}

// ---------- Mock: BuddyRepository ----------

type mockBuddyRepo struct {
	mu      sync.Mutex
	buddies []*domain.Buddy
	err     error
}

func (m *mockBuddyRepo) Create(_ context.Context, buddy *domain.Buddy) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buddies = append(m.buddies, buddy)
	return nil
}

func (m *mockBuddyRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Buddy, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.buddies {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, fmt.Errorf("buddy not found")
}

func (m *mockBuddyRepo) GetByName(_ context.Context, name string) (*domain.Buddy, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.buddies {
		if b.Name == name {
			return b, nil
		}
	}
	return nil, fmt.Errorf("buddy not found: %s", name)
}

func (m *mockBuddyRepo) GetByNames(_ context.Context, names []string) ([]*domain.Buddy, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	var result []*domain.Buddy
	for _, b := range m.buddies {
		if nameSet[b.Name] {
			result = append(result, b)
		}
	}
	return result, nil
}

func (m *mockBuddyRepo) List(_ context.Context, _ shared.OffsetPage) ([]*domain.Buddy, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*domain.Buddy, len(m.buddies))
	copy(result, m.buddies)
	return result, nil
}

func (m *mockBuddyRepo) ListActive(_ context.Context, _ shared.OffsetPage) ([]*domain.Buddy, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Buddy
	for _, b := range m.buddies {
		if b.IsActive {
			result = append(result, b)
		}
	}
	return result, nil
}

func (m *mockBuddyRepo) Update(_ context.Context, buddy *domain.Buddy) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, b := range m.buddies {
		if b.ID == buddy.ID {
			m.buddies[i] = buddy
			return nil
		}
	}
	return fmt.Errorf("buddy not found")
}

func (m *mockBuddyRepo) DeleteByName(_ context.Context, name string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, b := range m.buddies {
		if b != nil && b.Name == name {
			m.buddies = append(m.buddies[:i], m.buddies[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("buddy not found")
}

func (m *mockBuddyRepo) Delete(_ context.Context, id uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, b := range m.buddies {
		if b.ID == id {
			m.buddies = append(m.buddies[:i], m.buddies[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("buddy not found")
}

// ---------- Mock: ChannelRepository ----------

type mockChannelRepo struct {
	mu       sync.Mutex
	channels []*domain.Channel
	err      error
}

func (m *mockChannelRepo) Create(_ context.Context, ch *domain.Channel) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels = append(m.channels, ch)
	return nil
}

func (m *mockChannelRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Channel, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.channels {
		if ch.ID == id {
			return ch, nil
		}
	}
	return nil, fmt.Errorf("channel not found")
}

func (m *mockChannelRepo) GetByName(_ context.Context, name string) (*domain.Channel, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.channels {
		if ch.Name == name {
			return ch, nil
		}
	}
	return nil, fmt.Errorf("channel not found")
}

func (m *mockChannelRepo) List(_ context.Context, _ shared.OffsetPage) ([]*domain.Channel, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*domain.Channel, len(m.channels))
	copy(result, m.channels)
	return result, nil
}

func (m *mockChannelRepo) ListActive(_ context.Context, _ shared.OffsetPage) ([]*domain.Channel, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Channel
	for _, ch := range m.channels {
		if !ch.IsArchived() {
			result = append(result, ch)
		}
	}
	if result == nil {
		result = []*domain.Channel{}
	}
	return result, nil
}

func (m *mockChannelRepo) Update(_ context.Context, ch *domain.Channel) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *mockChannelRepo) Archive(_ context.Context, _ uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *mockChannelRepo) Delete(_ context.Context, _ uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

// GetWorkspaceSlug satisfies the ChannelRepository interface so the
// mock keeps compiling against pkg/repository/interfaces.go. Tests
// that don't exercise the workspace-slug-resolution path get a
// stub-empty result, which matches the production behaviour for a
// not-yet-resolved workspace.
func (m *mockChannelRepo) GetWorkspaceSlug(_ context.Context, _ uuid.UUID) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "", nil
}

// ---------- Mock: ReactionRepository ----------

type mockReactionRepo struct {
	mu        sync.Mutex
	reactions []*reaction.Reaction
	nextID    reaction.ReactionID
	err       error
}

func (m *mockReactionRepo) Add(_ context.Context, r *reaction.Reaction) (reaction.ReactionID, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	r.ID = m.nextID
	m.reactions = append(m.reactions, r)
	return r.ID, nil
}

func (m *mockReactionRepo) Remove(_ context.Context, messageID message.MessageID, userID string, emoji string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.reactions {
		if r.MessageID == messageID && string(r.UserID) == userID && r.Emoji == emoji {
			m.reactions = append(m.reactions[:i], m.reactions[i+1:]...)
			return nil
		}
	}
	return nil // idempotent
}

func (m *mockReactionRepo) ListByMessage(_ context.Context, messageID message.MessageID) ([]*reaction.Reaction, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*reaction.Reaction
	for _, r := range m.reactions {
		if r.MessageID == messageID {
			result = append(result, r)
		}
	}
	if result == nil {
		result = []*reaction.Reaction{}
	}
	return result, nil
}

func (m *mockReactionRepo) ListByMessages(_ context.Context, messageIDs []message.MessageID) (map[message.MessageID][]*reaction.Reaction, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[message.MessageID][]*reaction.Reaction)
	idSet := make(map[message.MessageID]bool)
	for _, id := range messageIDs {
		idSet[id] = true
	}
	for _, r := range m.reactions {
		if idSet[r.MessageID] {
			result[r.MessageID] = append(result[r.MessageID], r)
		}
	}
	return result, nil
}

func (m *mockReactionRepo) GetByID(_ context.Context, id reaction.ReactionID) (*reaction.Reaction, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.reactions {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

// ---------- Mock: HeartbeatRepository ----------

// ---------- Mock: CronJobRepository ----------

type mockCronJobRepo struct {
	mu   sync.Mutex
	jobs []*domain.CronJob
	err  error
}

func (m *mockCronJobRepo) Create(_ context.Context, job *domain.CronJob) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *mockCronJobRepo) Update(_ context.Context, job *domain.CronJob) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *mockCronJobRepo) Delete(_ context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *mockCronJobRepo) GetByID(_ context.Context, id string) (*domain.CronJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return nil, fmt.Errorf("cron job not found")
}

func (m *mockCronJobRepo) List(_ context.Context, workspaceID string) ([]*domain.CronJob, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.CronJob
	for _, job := range m.jobs {
		if job.WorkspaceID == workspaceID {
			result = append(result, job)
		}
	}
	if result == nil {
		result = []*domain.CronJob{}
	}
	return result, nil
}

// ---------- Mock: CronHistoryRepository ----------

type mockCronHistoryRepo struct {
	mu      sync.Mutex
	records []*domain.CronRunRecord
	err     error
}

func (m *mockCronHistoryRepo) Record(_ context.Context, record *domain.CronRunRecord) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, record)
	return nil
}

func (m *mockCronHistoryRepo) ListByJob(_ context.Context, jobID string, limit int) ([]*domain.CronRunRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.CronRunRecord
	for _, r := range m.records {
		if r.JobID == jobID {
			result = append(result, r)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	if result == nil {
		result = []*domain.CronRunRecord{}
	}
	return result, nil
}

func (m *mockCronHistoryRepo) ListRecent(_ context.Context, limit int) ([]*domain.CronRunRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*domain.CronRunRecord, len(m.records))
	copy(result, m.records)
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockCronHistoryRepo) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return 0, nil
}

// ---------- Mock: AgentSecretRepository ----------

type mockAgentSecretRepo struct {
	mu      sync.Mutex
	secrets map[string]map[string]string // agentID -> name -> value
	err     error
}

func newMockAgentSecretRepo() *mockAgentSecretRepo {
	return &mockAgentSecretRepo{
		secrets: make(map[string]map[string]string),
	}
}

func (m *mockAgentSecretRepo) Set(_ context.Context, agentID, name, value, _ string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secrets[agentID] == nil {
		m.secrets[agentID] = make(map[string]string)
	}
	m.secrets[agentID][name] = value
	return nil
}

func (m *mockAgentSecretRepo) Get(_ context.Context, agentID, name string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secrets[agentID] == nil {
		return "", fmt.Errorf("secret not found")
	}
	v, ok := m.secrets[agentID][name]
	if !ok {
		return "", fmt.Errorf("secret not found")
	}
	return v, nil
}

func (m *mockAgentSecretRepo) List(_ context.Context, agentID string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for name := range m.secrets[agentID] {
		names = append(names, name)
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func (m *mockAgentSecretRepo) Delete(_ context.Context, agentID, name string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secrets[agentID] != nil {
		delete(m.secrets[agentID], name)
	}
	return nil
}

func (m *mockAgentSecretRepo) DeleteAll(_ context.Context, agentID string) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.secrets, agentID)
	return nil
}

func (m *mockAgentSecretRepo) Count(_ context.Context, agentID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.secrets[agentID]), nil
}

// ---------- Mock: MessageRepository ----------

type mockMessageRepo struct {
	mu       sync.Mutex
	messages []*message.Message
	nextID   message.MessageID
	err      error
}

func (m *mockMessageRepo) Save(_ context.Context, msg *message.Message) (message.MessageID, error) {
	if m.err != nil {
		return 0, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	msg.ID = m.nextID
	m.messages = append(m.messages, msg)
	return msg.ID, nil
}

func (m *mockMessageRepo) FindByID(_ context.Context, id message.MessageID) (*message.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.messages {
		if msg.ID == id {
			return msg, nil
		}
	}
	return nil, fmt.Errorf("message not found")
}

func (m *mockMessageRepo) ListByChannel(_ context.Context, _ string, _ shared.CursorPage) (*message.PaginatedMessages, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &message.PaginatedMessages{Messages: []*message.Message{}, HasMore: false}, nil
}

func (m *mockMessageRepo) ListReplies(_ context.Context, _ message.MessageID) ([]*message.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*message.Message{}, nil
}

func (m *mockMessageRepo) ListByAuthor(_ context.Context, _ shared.ActorID, _ int) ([]*message.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*message.Message{}, nil
}

func (m *mockMessageRepo) ListSince(_ context.Context, _ string, _ message.MessageID) ([]*message.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*message.Message{}, nil
}

func (m *mockMessageRepo) MarkAsRead(_ context.Context, _ message.MessageID) error {
	return m.err
}

func (m *mockMessageRepo) MarkChannelAsRead(_ context.Context, _ string) error {
	return m.err
}

func (m *mockMessageRepo) UpdateContent(_ context.Context, _ *message.Message) error {
	return m.err
}

func (m *mockMessageRepo) CountUnreadMentionsByChannel(_ context.Context, _ shared.ActorID) (map[string]int, error) {
	if m.err != nil {
		return nil, m.err
	}
	return map[string]int{}, nil
}

func (m *mockMessageRepo) ListUnreadMentions(_ context.Context, _ shared.ActorID, _ int) ([]*message.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*message.Message{}, nil
}

func (m *mockMessageRepo) ListUnreadThreadReplies(_ context.Context, _ shared.ActorID, _ int) ([]*message.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*message.Message{}, nil
}

// ---------- Mock: MembershipRepository ----------

type mockMembershipRepo struct {
	mu          sync.Mutex
	memberships []*channel.ChannelMembership
	err         error
}

func (m *mockMembershipRepo) Save(_ context.Context, membership *channel.ChannelMembership) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberships = append(m.memberships, membership)
	return nil
}

func (m *mockMembershipRepo) FindByID(_ context.Context, id channel.MembershipID) (*channel.ChannelMembership, error) {
	if m.err != nil {
		return nil, m.err
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockMembershipRepo) IsMember(_ context.Context, _ channel.ChannelID, _ shared.ActorID) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return true, nil
}

func (m *mockMembershipRepo) IsAdmin(_ context.Context, _ channel.ChannelID, _ shared.ActorID) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return false, nil
}

func (m *mockMembershipRepo) ListByChannel(_ context.Context, _ channel.ChannelID, _ shared.OffsetPage) ([]*channel.ChannelMembership, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*channel.ChannelMembership{}, nil
}

func (m *mockMembershipRepo) ListByActor(_ context.Context, _ shared.ActorID) ([]*channel.ChannelMembership, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*channel.ChannelMembership{}, nil
}

func (m *mockMembershipRepo) CountAdmins(_ context.Context, _ channel.ChannelID) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return 1, nil
}

func (m *mockMembershipRepo) Delete(_ context.Context, _ channel.MembershipID) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

// ---------- Mock: Logger (for message.Service) ----------

type mockLogger struct{}

func (l *mockLogger) Info(_ string, _ ...any)  {}
func (l *mockLogger) Warn(_ string, _ ...any)  {}
func (l *mockLogger) Error(_ string, _ ...any) {}
func (l *mockLogger) Debug(_ string, _ ...any) {}

// ---------- Helpers ----------

// newTestChatServer creates a ChatServer with all mock repos wired up for testing.
// No real database, no real services. The messageService uses mock repos.
func newTestChatServer() *ChatServer {
	buddyRepo := &mockBuddyRepo{}
	channelRepo := &mockChannelRepo{}
	reactionRepo := &mockReactionRepo{}
	cronRepo := &mockCronJobRepo{}
	cronHistoryRepo := &mockCronHistoryRepo{}
	agentSecretRepo := newMockAgentSecretRepo()
	membershipRepo := &mockMembershipRepo{}
	msgRepo := &mockMessageRepo{}

	// Build a real message.Service backed by mock repos
	msgService := message.NewService(
		&mockLogger{},
		msgRepo,
		membershipRepo,
		channelRepo,
		buddyRepo,
		nil, // authRepo - not needed for handler tests
		nil, // agentExecutor
		nil, // remoteExecutor
	)

	channelService := channel.NewChannelService(membershipRepo, nil)

	// Suppress slog output in tests
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	hub := websocket.NewHub()
	go hub.Run()

	cs := &ChatServer{
		hub:             hub,
		buddyRepo:       buddyRepo,
		channelRepo:     channelRepo,
		reactionRepo:    reactionRepo,
		cronRepo:        cronRepo,
		cronHistoryRepo: cronHistoryRepo,
		agentSecretRepo: agentSecretRepo,
		channelService:  channelService,
		messageService:  msgService,
		authzService:    nil, // nil = skip authorization checks
		statsCache:      newStatsCache(30 * time.Second),
	}

	return cs
}

// authenticatedRequest creates an *http.Request with the actorID injected into context.
func authenticatedRequest(method, path string, body io.Reader, actorID string) *http.Request {
	req := httptest.NewRequest(method, path, body)
	ctx := context.WithValue(req.Context(), sharedctx.ActorIDKey, shared.ActorID(actorID))
	return req.WithContext(ctx)
}
