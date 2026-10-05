package gateway

import (
	"encoding/json"
	"fmt"
	"memdoor/pkg/shared"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// generateUUID creates a new UUID string
func generateUUID() string {
	return uuid.New().String()
}

// handleGetCSRFToken handles GET /api/auth/csrf
// Returns a CSRF token for the current session (web UI protection)
func (s *Server) handleGetCSRFToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session ID from cookie or create new one
	sessionCookie, err := r.Cookie("session_id")
	var sessionID string
	if err != nil {
		// No session cookie yet - create one
		sessionID = generateUUID()
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   86400, // 24 hours
		})
	} else {
		sessionID = sessionCookie.Value
	}

	// Generate CSRF token for this session
	csrfToken, err := s.csrfManager.GenerateToken(sessionID)
	if err != nil {
		http.Error(w, "Failed to generate CSRF token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"csrf_token": csrfToken,
	})
}

// handleCLIAuth handles GET /api/auth/cli
// Dedicated endpoint for CLI authentication flow
// isLoopbackCallback reports whether the CLI auth callback URL targets a
// loopback host. The legitimate CLI flow spins up a localhost listener,
// so anything other than 127.0.0.1 / localhost / ::1 is rejected to
// prevent open-redirect token theft.
func isLoopbackCallback(callback string) bool { return shared.IsLoopbackURL(callback) }

func (s *Server) handleCLIAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get callback URL from query params
	callback := r.URL.Query().Get("callback")
	if callback == "" {
		http.Error(w, "Missing callback parameter", http.StatusBadRequest)
		return
	}

	// Open-redirect / token-theft guard: the CLI flow always points the
	// callback at a localhost listener it owns. Refuse any non-loopback
	// host so a crafted callback can't exfiltrate the auth token.
	if !isLoopbackCallback(callback) {
		http.Error(w, "invalid callback", http.StatusBadRequest)
		return
	}

	// Check if user is already authenticated via session cookie or Authorization header
	var token string

	// First, check for Authorization header (from API calls)
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		// Extract Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			token = parts[1]
		}
	}

	// If no Authorization header, check for session cookie
	if token == "" {
		cookie, err := r.Cookie("session_token")
		if err == nil && cookie.Value != "" {
			token = cookie.Value
		}
	}

	// If we have a token, verify it and redirect immediately
	if token != "" {
		_, err := s.authAdapter.ValidateToken(token)
		if err == nil {
			// Token is valid - redirect to callback with token
			redirectURL := fmt.Sprintf("%s?token=%s", callback, token)
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
		// Token invalid, fall through to login page
	}

	// No valid token - show login page with callback
	// This will redirect to the callback after successful login
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Build HTML with proper escaping
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>CLI Authentication - Memdoor</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .container {
            background: white;
            padding: 3rem;
            border-radius: 1rem;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            max-width: 400px;
            width: 100%;
        }
        h1 {
            color: #667eea;
            margin-bottom: 1rem;
            text-align: center;
        }
        .info {
            background: #e8f4fd;
            border-left: 4px solid #667eea;
            padding: 1rem;
            margin-bottom: 1.5rem;
            border-radius: 0.5rem;
        }
        .info p {
            margin: 0;
            color: #1e40af;
            font-size: 0.9rem;
        }
        .form-group {
            margin-bottom: 1.25rem;
        }
        label {
            display: block;
            margin-bottom: 0.5rem;
            color: #374151;
            font-weight: 500;
            font-size: 0.9rem;
        }
        input {
            width: 100%%;
            padding: 0.75rem;
            border: 1px solid #d1d5db;
            border-radius: 0.5rem;
            font-size: 1rem;
            box-sizing: border-box;
            color: #1f2937;
        }
        input::placeholder {
            color: #9ca3af;
        }
        input:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        button {
            width: 100%;
            padding: 0.875rem;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 0.5rem;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s;
        }
        button:hover {
            transform: translateY(-1px);
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
        }
        button:disabled {
            opacity: 0.6;
            cursor: not-allowed;
        }
        .error {
            background: #fee;
            color: #c00;
            padding: 0.75rem;
            border-radius: 0.5rem;
            margin-bottom: 1rem;
            font-size: 0.9rem;
            display: none;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 CLI Authentication</h1>

        <div class="info">
            <p>🖥️ Authenticating for CLI access</p>
        </div>

        <div id="error" class="error"></div>

        <form id="loginForm">
            <div class="form-group">
                <label for="email">Email</label>
                <input type="email" id="email" name="email" placeholder="you@example.com" required>
            </div>

            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" placeholder="••••••••" required>
            </div>

            <button type="submit" id="submitBtn">Login</button>
        </form>
    </div>

    <script>
        const form = document.getElementById('loginForm');
        const errorDiv = document.getElementById('error');
        const submitBtn = document.getElementById('submitBtn');

        // Check if already logged in (token in localStorage)
        const existingToken = localStorage.getItem('auth_token');
        if (existingToken) {
            // Already authenticated - redirect immediately
            window.location.href = 'CALLBACK_URL?token=' + encodeURIComponent(existingToken);
        }

        form.addEventListener('submit', async (e) => {
            e.preventDefault();
            errorDiv.style.display = 'none';
            submitBtn.disabled = true;
            submitBtn.textContent = 'Logging in...';

            const email = document.getElementById('email').value;
            const password = document.getElementById('password').value;

            try {
                const response = await fetch('/api/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ email, password })
                });

                if (!response.ok) {
                    const data = await response.json();
                    throw new Error(data.error || 'Login failed');
                }

                const data = await response.json();

                // Redirect to callback with token
                window.location.href = 'CALLBACK_URL?token=' + encodeURIComponent(data.token);
            } catch (err) {
                errorDiv.textContent = err.message;
                errorDiv.style.display = 'block';
                submitBtn.disabled = false;
                submitBtn.textContent = 'Login';
            }
        });
    </script>
</body>
</html>`

	// Replace placeholder with actual callback URL
	html = strings.Replace(html, "CALLBACK_URL", callback, -1)

	// Write the HTML
	fmt.Fprint(w, html)
}
