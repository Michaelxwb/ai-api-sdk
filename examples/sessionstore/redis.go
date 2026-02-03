package sessionstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/session"
	"github.com/redis/go-redis/v9"
)

// RedisStore implements a Redis-backed session store for high concurrency scenarios.
type RedisStore struct {
	rdb    *redis.Client
	Prefix string
	TTL    time.Duration
}

// NewRedisStore creates a new Redis-backed store.
// ttl=0 disables expiration.
func NewRedisStore(rdb *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{
		rdb:    rdb,
		Prefix: "session:",
		TTL:    ttl,
	}
}

// Get retrieves a full session state.
func (s *RedisStore) Get(ctx context.Context, sessionID string) (*session.SessionState, error) {
	var meta *session.SessionMeta
	metaPayload, err := s.rdb.Get(ctx, s.metaKey(sessionID)).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			return nil, err
		}
	} else {
		var stored session.SessionMeta
		if err := json.Unmarshal(metaPayload, &stored); err != nil {
			return nil, err
		}
		meta = &stored
	}

	items, err := s.rdb.LRange(ctx, s.messagesKey(sessionID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && meta == nil {
		return nil, session.ErrSessionNotFound
	}

	msgs := make([]session.Message, 0, len(items))
	for _, item := range items {
		var msg session.Message
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}

	state := &session.SessionState{
		ID:       sessionID,
		Messages: msgs,
	}
	applyStoredMeta(state, meta)
	return state, nil
}

// Save writes the provided session state.
func (s *RedisStore) Save(ctx context.Context, state *session.SessionState) error {
	if state == nil {
		return errors.New("session store: nil state")
	}
	if state.ID == "" {
		return errors.New("session store: missing session id")
	}

	now := time.Now()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	state.UpdatedAt = now

	// 拉 existing meta 用于合并
	var existingMeta *session.SessionMeta
	existingRaw, err := s.rdb.Get(ctx, s.metaKey(state.ID)).Bytes()
	if err == nil && len(existingRaw) > 0 {
		var em session.SessionMeta
		if json.Unmarshal(existingRaw, &em) == nil {
			existingMeta = &em
		}
	}

	meta := normalizeMetaForSave(state, existingMeta, now)
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	values := make([]interface{}, 0, len(state.Messages))
	for _, msg := range state.Messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		values = append(values, data)
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, s.metaKey(state.ID), payload, s.TTL)
	pipe.Del(ctx, s.messagesKey(state.ID))
	if len(values) > 0 {
		pipe.RPush(ctx, s.messagesKey(state.ID), values...)
	}
	pipe.Incr(ctx, s.versionKey(state.ID))
	if s.TTL > 0 {
		pipe.Expire(ctx, s.messagesKey(state.ID), s.TTL)
		pipe.Expire(ctx, s.versionKey(state.ID), s.TTL)
		pipe.Expire(ctx, s.metaKey(state.ID), s.TTL)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// Delete removes a session and all its messages.
func (s *RedisStore) Delete(ctx context.Context, sessionID string) error {
	return s.DeleteSession(ctx, sessionID)
}

// Append appends messages to a session.
func (s *RedisStore) Append(ctx context.Context, sessionID string, msgs ...session.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	values := make([]interface{}, 0, len(msgs))
	for _, msg := range msgs {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		values = append(values, data)
	}

	pipe := s.rdb.Pipeline()
	pipe.RPush(ctx, s.messagesKey(sessionID), values...)
	pipe.Incr(ctx, s.versionKey(sessionID))
	if s.TTL > 0 {
		pipe.Expire(ctx, s.messagesKey(sessionID), s.TTL)
		pipe.Expire(ctx, s.versionKey(sessionID), s.TTL)
		pipe.Expire(ctx, s.metaKey(sessionID), s.TTL)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// CreateSession creates a new session metadata entry.
func (s *RedisStore) CreateSession(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
	key := s.metaKey(sessionID)
	stored := session.SessionMeta{}
	applyMeta(&stored, sessionID, meta)
	payload, err := json.Marshal(stored)
	if err != nil {
		return err
	}

	ok, err := s.rdb.SetNX(ctx, key, payload, s.TTL).Result()
	if err != nil {
		return err
	}
	if !ok {
		return session.ErrSessionConflict
	}
	return nil
}

// DeleteSession deletes all session keys.
func (s *RedisStore) DeleteSession(ctx context.Context, sessionID string) error {
	count, err := s.rdb.Del(ctx, s.messagesKey(sessionID), s.metaKey(sessionID), s.versionKey(sessionID)).Result()
	if err != nil {
		return err
	}
	if count == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

// GetMeta retrieves session metadata.
func (s *RedisStore) GetMeta(ctx context.Context, sessionID string) (*session.SessionMeta, error) {
	data, err := s.rdb.Get(ctx, s.metaKey(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, session.ErrSessionNotFound
		}
		return nil, err
	}

	var meta session.SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// UpsertMeta updates or creates metadata.
func (s *RedisStore) UpsertMeta(ctx context.Context, sessionID string, meta *session.SessionMeta) error {
	key := s.metaKey(sessionID)
	current := session.SessionMeta{}
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err == nil {
		_ = json.Unmarshal(data, &current)
	} else if !errors.Is(err, redis.Nil) {
		return err
	}

	applyMeta(&current, sessionID, meta)
	payload, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, payload, s.TTL).Err()
}

// GetVersion returns the current session version.
func (s *RedisStore) GetVersion(ctx context.Context, sessionID string) (int64, error) {
	val, err := s.rdb.Get(ctx, s.versionKey(sessionID)).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

// AppendMessagesWithVersion appends messages with optimistic locking.
func (s *RedisStore) AppendMessagesWithVersion(ctx context.Context, sessionID string, expectedVersion int64, msgs []session.Message) (int64, error) {
	if len(msgs) == 0 {
		return expectedVersion, nil
	}

	keyVer := s.versionKey(sessionID)
	keyMsgs := s.messagesKey(sessionID)

	var newVersion int64
	err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
		current, err := tx.Get(ctx, keyVer).Int64()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				current = 0
			} else {
				return err
			}
		}
		if current != expectedVersion {
			return session.ErrSessionConflict
		}

		values := make([]interface{}, 0, len(msgs))
		for _, msg := range msgs {
			data, err := json.Marshal(msg)
			if err != nil {
				return err
			}
			values = append(values, data)
		}

		newVersion = expectedVersion + 1
		pipe := tx.TxPipeline()
		pipe.RPush(ctx, keyMsgs, values...)
		pipe.Set(ctx, keyVer, newVersion, 0)
		if s.TTL > 0 {
			pipe.Expire(ctx, keyMsgs, s.TTL)
			pipe.Expire(ctx, keyVer, s.TTL)
			pipe.Expire(ctx, s.metaKey(sessionID), s.TTL)
		}
		_, err = pipe.Exec(ctx)
		return err
	}, keyVer)

	if err != nil {
		if errors.Is(err, redis.TxFailedErr) || errors.Is(err, session.ErrSessionConflict) {
			return 0, session.ErrSessionConflict
		}
		return 0, err
	}

	return newVersion, nil
}

func (s *RedisStore) lookupCreatedAt(ctx context.Context, sessionID string) (time.Time, error) {
	data, err := s.rdb.Get(ctx, s.metaKey(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	var meta session.SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return time.Time{}, err
	}
	return meta.CreatedAt, nil
}

func (s *RedisStore) messagesKey(sessionID string) string {
	return s.Prefix + sessionID + ":messages"
}

func (s *RedisStore) metaKey(sessionID string) string {
	return s.Prefix + sessionID + ":meta"
}

func (s *RedisStore) versionKey(sessionID string) string {
	return s.Prefix + sessionID + ":version"
}

var (
	_ session.SessionStore         = (*RedisStore)(nil)
	_ session.SessionStoreAppender = (*RedisStore)(nil)
)
