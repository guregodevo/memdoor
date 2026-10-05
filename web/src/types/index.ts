// Actor ID (matches shared.ActorID from Go)
export type ActorID = `human:${string}` | `agent:${string}`;

export interface ActorInfo {
  id: ActorID;
  name: string;
  avatar?: string;
  isAgent: boolean;
  status?: 'online' | 'offline' | 'busy';
}

// Channel types (conversation spaces like #general, #test2)
export interface Channel {
  id: string;
  name: string;
  type: 'public' | 'private' | 'dm';
  members: ActorID[];
  created_at: string;
  updated_at: string;
}

// Reaction types
export interface Reaction {
  id: number;
  user_id: ActorID;
  emoji: string;
  created_at: string;
}

// Message types
export interface Message {
  id: string; // Changed from number to string to match backend DTO
  channel_id: string;
  author_id: ActorID;
  author_name: string; // Username for the author (e.g., "alice" for humans, "writer" for agents)
  content: MessageContent;
  reactions?: Reaction[]; // Message reactions
  parent_id?: number; // If this is a thread reply, ID of the parent message
  is_read: boolean;
  created_at: string;
  updated_at: string;
  execution_id?: string; // If message triggered agent execution
}

export interface FileAttachment {
  id: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  url: string;
}

export interface MessageContent {
  text: string;
  mentions: ActorID[];
  link_previews?: LinkPreview[];
  attachments?: FileAttachment[];
}

export interface LinkPreview {
  url: string;
  type: 'message' | 'url' | 'image';
  message_data?: {
    message_id: string;
    channel_id: string;
    channel_name: string;
    author_id: ActorID;
    author_name: string;
    text: string;
    created_at: string;
  };
}

// Event types (matches shared.Event from Go)
export type EventType =
  | 'execution.started'
  | 'execution.completed'
  | 'execution.failed'
  | 'thinking.started'
  | 'thinking.completed'
  | 'tool.call.started'
  | 'tool.call.completed'
  | 'tool.call.failed'
  | 'response.generated'
  | 'error.occurred';

export type EventCategory = 'lifecycle' | 'thinking' | 'tool' | 'response' | 'error';

export interface PlatformEvent {
  type: EventType;
  category: EventCategory;
  data: Record<string, any>;
  timestamp: number; // Unix milliseconds
}

// User types
export interface User {
  id: ActorID;
  username: string; // Unique @mention username (e.g., "alice")
  name: string;     // Display name (e.g., "Alice Smith")
  email: string;
  avatar_url?: string;
  status?: 'online' | 'offline' | 'busy';
}

// Agent types
export interface Agent {
  id: string;
  name: string;
  avatar_emoji?: string;
  icon?: string; // Animal mascot icon (fox, owl, panda, etc.)
  profile: 'minimal' | 'coding' | 'messaging' | 'full';
  model: string;
  system_prompt?: string;
  tools: string[];
  status: 'online' | 'offline' | 'busy';
}

// WebSocket message types
export type WSMessageType =
  | 'message.created'
  | 'message.updated'
  | 'message.deleted'
  | 'reaction.added'
  | 'reaction.removed'
  | 'execution.started'
  | 'execution.event'
  | 'execution.completed'
  | 'execution.failed'
  | 'execution.todo.updated'
  | 'typing.started'
  | 'typing.stopped'
  | 'presence.updated'
  | 'mention.created';

export interface WSMessage {
  type: WSMessageType;
  data: any;
}

// Presence update types (real-time online/offline status)
export interface PresenceUpdate {
  actor_id: ActorID;
  status: 'online' | 'offline' | 'busy';
  timestamp: number;
}

// Agent Todo types
export interface AgentTodo {
  content: string;
  status: 'pending' | 'in_progress' | 'completed';
  activeForm: string; // e.g., "Running tests", "Building project"
}

export interface AgentTodoState {
  executionId: string;
  agentId: string;
  channelId: string;
  todos: AgentTodo[];
  lastUpdated: number; // Unix timestamp
}

// Search types (matches pkg/search domain model)
export type SearchResultType = 'message' | 'file' | 'url';

export interface SearchResult {
  id: string;
  type: SearchResultType;
  score: number; // Relevance score 0.0-1.0
  content: string; // Preview of the content
  channel_id: string;
  channel_name: string;
  source_id: string;
  author_id: ActorID;
  author_name: string;
  created_at: string;
  metadata?: Record<string, any>;
}

export interface SearchResponse {
  results: SearchResult[];
  query: string;
  total_results: number;
}

