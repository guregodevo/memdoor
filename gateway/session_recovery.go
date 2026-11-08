package gateway

import (
	"fmt"
	"time"

	"memdoor/pkg/llm"
)

func copyConversation(messages []llm.MessageParam) []llm.MessageParam {
	copy := append([]llm.MessageParam(nil), messages...)
	for i := range copy {
		copy[i].Content = append([]llm.ContentBlockParamUnion(nil), copy[i].Content...)
	}
	return copy
}

func (s *Session) pendingConversation(key string) ([]llm.MessageParam, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	messages, ok := s.pendingSaves[key]
	return copyConversation(messages), ok
}

func (s *Session) keepPendingConversation(key string, messages []llm.MessageParam) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingSaves == nil {
		s.pendingSaves = make(map[string][]llm.MessageParam)
	}
	s.pendingSaves[key] = copyConversation(messages)
}

func (s *Session) clearPendingConversation(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingSaves, key)
}

// sessionCursor counts only messages committed to the transcript.
func sessionCursor(session *Session, n int) int {
	raw, _ := session.GetMetadataValue("conversation_length")
	cursor, _ := raw.(int)
	return max(0, min(cursor, n))
}

func saveConversationTail(sp *SessionPersistence, session *Session, key string, conversation []llm.MessageParam) error {
	var err error
	if sp == nil {
		err = fmt.Errorf("conversation storage is unavailable")
	} else if unsaved := conversation[sessionCursor(session, len(conversation)):]; len(unsaved) > 0 {
		err = sp.SaveMessages(key, unsaved)
	}
	if err != nil {
		session.keepPendingConversation(key, conversation)
		return err
	}
	session.SetMetadata("conversation_length", len(conversation))
	session.SetMetadata("last_updated", time.Now())
	session.clearPendingConversation(key)
	return nil
}

func saveConversationReshape(sp *SessionPersistence, session *Session, key string, before, after []llm.MessageParam) error {
	var err error
	if sp == nil {
		err = fmt.Errorf("conversation storage is unavailable")
	} else {
		err = sp.SaveReshapedConversation(key, before[sessionCursor(session, len(before)):], after)
	}
	if err != nil {
		session.keepPendingConversation(key, before)
		return err
	}
	session.SetMetadata("conversation_length", len(after))
	session.clearPendingConversation(key)
	return nil
}
