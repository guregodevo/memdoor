package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"memdoor/internal/testenv"
	"os"
	"path/filepath"
	"testing"

	"memdoor/pkg/llm"
)

func checkpointStore(t *testing.T) (*SessionPersistence, *Session, string) {
	t.Helper()
	testenv.SetHome(t, t.TempDir())
	testenv.Logger(t)
	sp, err := NewSessionPersistence(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return sp, &Session{ID: "checkpoint", Metadata: make(map[string]interface{})}, "checkpoint"
}

func TestSessionSaveFailureRetainsCursorAndRetriesWithoutDuplicates(t *testing.T) {
	sp, session, key := checkpointStore(t)
	first := llm.NewUserMessage(llm.NewTextBlock("first"))
	conversation := []llm.MessageParam{first}
	if err := saveConversationTail(sp, session, key, conversation); err != nil {
		t.Fatal(err)
	}
	path, _ := sp.resolveSessionStorePath(key)
	if err := os.Rename(path, path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	conversation = append(conversation, llm.NewAssistantMessage(llm.NewTextBlock("unsaved answer")))
	if err := saveConversationTail(sp, session, key, conversation); err == nil {
		t.Fatal("storage failure was hidden")
	}
	if got := sessionCursor(session, 10); got != 1 {
		t.Fatalf("failed write advanced cursor to %d", got)
	}
	pending, ok := session.pendingConversation(key)
	if !ok || len(pending) != 2 {
		t.Fatalf("unsaved conversation lost: %v", pending)
	}
	// Simulate another turn rebuilding from the retained view.
	pending = append(pending, llm.NewUserMessage(llm.NewTextBlock("retry")))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path+".saved", path); err != nil {
		t.Fatal(err)
	}
	if err := saveConversationTail(sp, session, key, pending); err != nil {
		t.Fatal(err)
	}
	// An identical retry must also be harmless.
	if err := saveConversationTail(sp, session, key, pending); err != nil {
		t.Fatal(err)
	}
	history, err := sp.LoadMessages(key)
	if err != nil || len(history) != 3 || sessionCursor(session, 10) != 3 {
		t.Fatalf("retry duplicated or lost messages: history=%v err=%v", history, err)
	}
	if _, ok := session.pendingConversation(key); ok {
		t.Fatal("successful retry retained a stale pending conversation")
	}
}

func TestFailedReshapeKeepsOriginalHistoryAndRecoveryView(t *testing.T) {
	sp, session, key := checkpointStore(t)
	before := []llm.MessageParam{llm.NewUserMessage(llm.NewTextBlock("first"))}
	if err := saveConversationTail(sp, session, key, before); err != nil {
		t.Fatal(err)
	}
	path, _ := sp.resolveSessionStorePath(key)
	original, _ := os.ReadFile(path)
	before = append(before, llm.NewUserMessage(llm.NewTextBlock("new question")))
	invalid := llm.MessageParam{Role: "assistant", Content: []llm.ContentBlockParamUnion{{
		OfToolUse: &llm.ToolUseBlockParam{Type: "tool_use", ID: "bad", Name: "bash", Input: json.RawMessage("{")},
	}}}
	summary := llm.NewUserMessage(llm.NewTextBlock("summary"))
	if err := saveConversationReshape(sp, session, key, before, []llm.MessageParam{summary, invalid}); err == nil {
		t.Fatal("invalid boundary was saved")
	}
	afterFailure, _ := os.ReadFile(path)
	pending, ok := session.pendingConversation(key)
	if !bytes.Equal(original, afterFailure) || !ok || len(pending) != 2 || sessionCursor(session, 10) != 1 {
		t.Fatal("failed reshape changed disk, cursor, or recovery view")
	}
	if err := saveConversationReshape(sp, session, key, pending, []llm.MessageParam{summary}); err != nil {
		t.Fatal(err)
	}
	history, err := sp.LoadMessages(key)
	recent, recentErr := sp.LoadRecentMessages(key, 10)
	if err != nil || recentErr != nil || len(history) != 2 || len(recent) != 1 {
		t.Fatalf("reshape retry lost history or boundary: history=%v recent=%v err=%v/%v", history, recent, err, recentErr)
	}
}

func TestSessionBatchFailureLeavesOriginalAndNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	want := errors.New("disk full")
	err := appendSessionBatch(path, func(w io.Writer) error {
		if _, err := w.Write([]byte("partial\n")); err != nil {
			return err
		}
		return want
	})
	content, _ := os.ReadFile(path)
	files, _ := os.ReadDir(dir)
	if !errors.Is(err, want) || string(content) != "original\n" || len(files) != 1 {
		t.Fatalf("failed append was committed: content=%q files=%v err=%v", content, files, err)
	}
}

type shortRecordWriter struct{}

func (shortRecordWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestSessionRecordReportsShortWrites(t *testing.T) {
	_, err := marshalAndWriteRecord(shortRecordWriter{}, MessageRecord{Message: llm.NewUserMessage(llm.NewTextBlock("question"))})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write reported success: %v", err)
	}
}
