package renderer

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	fileID    string
	createdAt time.Time
}

// ImageCache представляет потокобезопасный кэш Telegram FileID для сгенерированных изображений расписания
type ImageCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

var defaultImageCache = NewImageCache(24 * time.Hour)

// GetDefaultCache возвращает глобальный экземпляр кэша
func GetDefaultCache() *ImageCache {
	return defaultImageCache
}

// NewImageCache создает новый экземпляр кэша с заданным временем жизни записей
func NewImageCache(ttl time.Duration) *ImageCache {
	return &ImageCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

// GroupKey формирует ключ кэша для группы
func GroupKey(groupID int32, week time.Time, subgroup int) string {
	dateStr := "current"
	if !week.IsZero() {
		dateStr = week.Format("02.01.2006")
	}
	return fmt.Sprintf("group:%d:%s:%d", groupID, dateStr, subgroup)
}

// TeacherKey формирует ключ кэша для преподавателя
func TeacherKey(teacherName string, week time.Time) string {
	dateStr := "current"
	if !week.IsZero() {
		dateStr = week.Format("02.01.2006")
	}
	return fmt.Sprintf("teacher:%s:%s", teacherName, dateStr)
}

// GetFileID возвращает сохраненный Telegram FileID по ключу, если он существует и не протух
func (c *ImageCache) GetFileID(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}

	if c.ttl > 0 && time.Since(entry.createdAt) > c.ttl {
		return "", false
	}

	return entry.fileID, true
}

// SetFileID сохраняет Telegram FileID для ключа
func (c *ImageCache) SetFileID(key, fileID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		fileID:    fileID,
		createdAt: time.Now(),
	}
}

// InvalidateGroup инвалидирует все кэшированные изображения для указанной группы
func (c *ImageCache) InvalidateGroup(groupID int32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := fmt.Sprintf("group:%d:", groupID)
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

// InvalidateTeacher инвалидирует кэш для указанного преподавателя
func (c *ImageCache) InvalidateTeacher(teacherName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := fmt.Sprintf("teacher:%s:", teacherName)
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

// Clear полностью очищает кэш
func (c *ImageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]cacheEntry)
}
