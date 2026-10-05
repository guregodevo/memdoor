package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"memdoor/pkg/authorization"
	"memdoor/pkg/channel"
	"memdoor/pkg/filestore"
	"memdoor/pkg/shared"
)

// SetTokenVerifier injects the token verifier for file download auth via query param
func (cs *ChatServer) SetTokenVerifier(verifier authorization.AuthService) {
	cs.tokenVerifier = verifier
}

// SetFileService injects the file service (called during server setup)
func (cs *ChatServer) SetFileService(svc *filestore.Service) {
	cs.fileService = svc
}

// handleFileUpload handles POST /api/files (multipart file upload)
func (cs *ChatServer) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if cs.fileService == nil {
		http.Error(w, `{"error":"File sharing not configured"}`, http.StatusServiceUnavailable)
		return
	}

	actorID := authorization.GetActorID(r.Context())
	if actorID == "" {
		http.Error(w, `{"error":"Authentication required"}`, http.StatusUnauthorized)
		return
	}

	// Parse multipart form (max 25MB + overhead)
	if err := r.ParseMultipartForm(26 << 20); err != nil {
		http.Error(w, `{"error":"Failed to parse form"}`, http.StatusBadRequest)
		return
	}

	channelID := r.FormValue("channel_id")
	if channelID == "" {
		http.Error(w, `{"error":"channel_id is required"}`, http.StatusBadRequest)
		return
	}

	// Authorization: must be able to write to the channel
	if cs.authzService != nil {
		canWrite, err := cs.authzService.CanWriteToChannel(r.Context(), actorID, channel.ChannelID(channelID))
		if err != nil {
			http.Error(w, `{"error":"Authorization check failed"}`, http.StatusInternalServerError)
			return
		}
		if !canWrite {
			http.Error(w, `{"error":"Permission denied"}`, http.StatusForbidden)
			return
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"No file provided"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	attachment, err := cs.fileService.Upload(r.Context(), channelID, actorID, header.Filename, mimeType, file)
	if err != nil {
		getChatLogger().Error("File upload failed",
			slog.String("actor_id", actorID.String()),
			slog.String("error", err.Error()))
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	getChatLogger().Info("File uploaded",
		slog.String("file_id", attachment.ID),
		slog.String("filename", attachment.Filename),
		slog.Int64("size_bytes", attachment.SizeBytes),
		slog.String("channel_id", channelID))

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         attachment.ID,
		"filename":   attachment.Filename,
		"mime_type":  attachment.MimeType,
		"size_bytes": attachment.SizeBytes,
		"url":        fmt.Sprintf("/api/files/%s", attachment.ID),
	})
}

// handleFileByID handles GET /api/files/:id (download with auth check)
func (cs *ChatServer) handleFileByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if cs.fileService == nil {
		http.Error(w, `{"error":"File sharing not configured"}`, http.StatusServiceUnavailable)
		return
	}

	// Extract file ID from path: /api/files/{id}
	fileID := strings.TrimPrefix(r.URL.Path, "/api/files/")
	if fileID == "" {
		http.Error(w, `{"error":"File ID required"}`, http.StatusBadRequest)
		return
	}

	// Support ?token= query param for <img> tags that can't set Authorization header
	actorID := authorization.GetActorID(r.Context())
	if actorID == "" {
		if tokenParam := r.URL.Query().Get("token"); tokenParam != "" {
			if cs.tokenVerifier != nil {
				user, err := cs.tokenVerifier.VerifyToken(tokenParam)
				if err == nil {
					actorID = shared.ActorID(user.ID)
				}
			}
		}
	}
	if actorID == "" {
		http.Error(w, `{"error":"Authentication required"}`, http.StatusUnauthorized)
		return
	}

	attachment, reader, err := cs.fileService.GetFile(r.Context(), fileID)
	if err != nil {
		http.Error(w, `{"error":"File not found"}`, http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Authorization: must be able to read the channel (avatars are public)
	if cs.authzService != nil && attachment.ChannelID != "__avatars__" {
		canRead, err := cs.authzService.CanReadChannel(r.Context(), actorID, channel.ChannelID(attachment.ChannelID))
		if err != nil || !canRead {
			http.Error(w, `{"error":"Permission denied"}`, http.StatusForbidden)
			return
		}
	}

	// Set headers for download
	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
	if attachment.IsImage() {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, attachment.Filename))
		w.Header().Set("Cache-Control", "private, max-age=86400") // 24h for images
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, attachment.Filename))
		w.Header().Set("Cache-Control", "private, max-age=3600") // 1h for other files
	}

	io.Copy(w, reader)
}
