package auth

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/s4f4y4t/go-microservice/pkg/apperror"
	"github.com/s4f4y4t/go-microservice/pkg/token"
)

const refreshKeyPrefix = "refresh:"

type RedisTokenStore struct {
	rdb *redis.Client
}

func NewRedisTokenStore(rdb *redis.Client) token.Store {
	return &RedisTokenStore{rdb: rdb}
}

func (s *RedisTokenStore) Save(ctx context.Context, tok string, userID int, expiry time.Duration) error {
	return s.rdb.Set(ctx, refreshKeyPrefix+tok, userID, expiry).Err()
}

func (s *RedisTokenStore) UserID(ctx context.Context, tok string) (int, error) {
	val, err := s.rdb.Get(ctx, refreshKeyPrefix+tok).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, apperror.Unauthorized("invalid or expired refresh token")
		}
		return 0, apperror.Internal(err)
	}
	id, err := strconv.Atoi(val)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return id, nil
}

func (s *RedisTokenStore) Delete(ctx context.Context, tok string) error {
	return s.rdb.Del(ctx, refreshKeyPrefix+tok).Err()
}
