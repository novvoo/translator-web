package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	SessionCookieName = "epub_session_id"
	SessionTimeout    = 24 * time.Hour
)

type Session struct {
	ID        string
	CreatedAt time.Time
	LastSeen  time.Time
}

type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var manager *SessionManager

func init() {
	manager = &SessionManager{
		sessions: make(map[string]*Session),
	}
	// 启动清理过期会话的协程
	go manager.cleanupExpiredSessions()
}

// generateSessionID 生成随机会话 ID
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetOrCreateSession 获取或创建会话
func (sm *SessionManager) GetOrCreateSession(sessionID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 如果提供了有效的 sessionID，尝试获取
	if sessionID != "" {
		if session, exists := sm.sessions[sessionID]; exists {
			// 检查会话是否过期
			if time.Since(session.LastSeen) < SessionTimeout {
				session.LastSeen = time.Now()
				return session
			}
			// 会话过期，删除
			delete(sm.sessions, sessionID)
		}
	}

	// 创建新会话
	newSession := &Session{
		ID:        generateSessionID(),
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}
	sm.sessions[newSession.ID] = newSession
	return newSession
}

// GetSession 获取会话（不创建新会话）
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(session.LastSeen) >= SessionTimeout {
		return nil, false
	}

	return session, true
}

// UpdateLastSeen 更新会话最后访问时间
func (sm *SessionManager) UpdateLastSeen(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.LastSeen = time.Now()
	}
}

// DeleteSession 删除会话
func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// cleanupExpiredSessions 定期清理过期会话
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, session := range sm.sessions {
			if now.Sub(session.LastSeen) >= SessionTimeout {
				delete(sm.sessions, id)
			}
		}
		sm.mu.Unlock()
	}
}

// SessionMiddleware Gin 中间件：确保每个请求都有会话
func SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Cookie 获取会话 ID
		sessionID, _ := c.Cookie(SessionCookieName)

		// 获取或创建会话
		session := manager.GetOrCreateSession(sessionID)

		// 设置 Cookie（如果是新会话或会话 ID 变化）
		if sessionID != session.ID {
			c.SetCookie(
				SessionCookieName,
				session.ID,
				int(SessionTimeout.Seconds()),
				"/",
				"",
				false, // secure (在生产环境应设为 true)
				true,  // httpOnly
			)
		}

		// 将会话 ID 存储到上下文
		c.Set("sessionID", session.ID)

		c.Next()
	}
}

// GetSessionID 从上下文获取会话 ID
func GetSessionID(c *gin.Context) string {
	sessionID, exists := c.Get("sessionID")
	if !exists {
		return ""
	}
	return sessionID.(string)
}
