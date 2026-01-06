package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	SessionCookieName = "session_id"
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
	if _, err := rand.Read(b); err != nil {
		// 使用加密安全的替代方案
		h := sha256.New()
		h.Write([]byte(time.Now().String()))
		h.Write([]byte(os.Getenv("HOSTNAME")))
		// 添加进程ID增加随机性
		h.Write([]byte(fmt.Sprintf("%d", os.Getpid())))
		return hex.EncodeToString(h.Sum(nil))
	}
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
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(session.LastSeen) >= SessionTimeout {
		// 删除过期会话
		delete(sm.sessions, sessionID)
		return nil, false
	}

	// 更新最后访问时间
	session.LastSeen = time.Now()
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
			// 根据请求协议设置secure标志
			isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
			c.SetCookie(
				SessionCookieName,
				session.ID,
				int(SessionTimeout.Seconds()),
				"/",
				"",
				isSecure, // 在HTTPS环境下启用secure标志
				true,     // httpOnly
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
	if id, ok := sessionID.(string); ok {
		return id
	}
	return ""
}
