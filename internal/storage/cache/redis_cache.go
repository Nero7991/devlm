package cache

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr string, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 5,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  300 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return err
	}

	compressedValue, err := compressValue(jsonValue)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key, compressedValue, expiration).Err()
}

func (c *RedisCache) MSet(ctx context.Context, values map[string]interface{}, expiration time.Duration) error {
	pipe := c.client.Pipeline()
	for key, value := range values {
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return err
		}

		compressedValue, err := compressValue(jsonValue)
		if err != nil {
			return err
		}

		pipe.Set(ctx, key, compressedValue, expiration)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // Key does not exist
		}
		return err
	}

	decompressedValue, err := decompressValue([]byte(val))
	if err != nil {
		return err
	}

	return json.Unmarshal(decompressedValue, dest)
}

func (c *RedisCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	results, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	decompressedResults := make([]interface{}, len(results))
	for i, result := range results {
		if result == nil {
			continue
		}
		decompressedValue, err := decompressValue([]byte(result.(string)))
		if err != nil {
			return nil, err
		}
		decompressedResults[i] = decompressedValue
	}

	return decompressedResults, nil
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) MDelete(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Del(ctx, keys...).Result()
}

func (c *RedisCache) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.client.Wait(1, 0).Err()
	if err != nil {
		return err
	}

	return c.client.Close()
}

func (c *RedisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return c.client.Exists(ctx, keys...).Result()
}

func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	jsonValue, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	compressedValue, err := compressValue(jsonValue)
	if err != nil {
		return false, err
	}

	return c.client.SetNX(ctx, key, compressedValue, expiration).Result()
}

func (c *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c *RedisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, key, value).Result()
}

func (c *RedisCache) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	return c.client.IncrByFloat(ctx, key, value).Result()
}

func (c *RedisCache) HSet(ctx context.Context, key string, values ...interface{}) error {
	compressedValues := make([]interface{}, len(values))
	for i := 0; i < len(values); i += 2 {
		field := values[i]
		value := values[i+1]

		jsonValue, err := json.Marshal(value)
		if err != nil {
			return err
		}

		compressedValue, err := compressValue(jsonValue)
		if err != nil {
			return err
		}

		compressedValues[i] = field
		compressedValues[i+1] = compressedValue
	}

	return c.client.HSet(ctx, key, compressedValues...).Err()
}

func (c *RedisCache) HGet(ctx context.Context, key string, field string, dest interface{}) error {
	val, err := c.client.HGet(ctx, key, field).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // Field does not exist
		}
		return err
	}

	decompressedValue, err := decompressValue([]byte(val))
	if err != nil {
		return err
	}

	return json.Unmarshal(decompressedValue, dest)
}

func (c *RedisCache) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	results, err := c.client.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}

	decompressedResults := make([]interface{}, len(results))
	for i, result := range results {
		if result == nil {
			continue
		}
		decompressedValue, err := decompressValue([]byte(result.(string)))
		if err != nil {
			return nil, err
		}
		decompressedResults[i] = decompressedValue
	}

	return decompressedResults, nil
}

func (c *RedisCache) HDelete(ctx context.Context, key string, fields ...string) (int64, error) {
	return c.client.HDel(ctx, key, fields...).Result()
}

func (c *RedisCache) HLen(ctx context.Context, key string) (int64, error) {
	return c.client.HLen(ctx, key).Result()
}

func (c *RedisCache) HGetAll(ctx context.Context, key string) (map[string]interface{}, error) {
	result, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	decompressedResult := make(map[string]interface{})
	for field, value := range result {
		decompressedValue, err := decompressValue([]byte(value))
		if err != nil {
			return nil, err
		}
		var unmarshaledValue interface{}
		err = json.Unmarshal(decompressedValue, &unmarshaledValue)
		if err != nil {
			return nil, err
		}
		decompressedResult[field] = unmarshaledValue
	}

	return decompressedResult, nil
}

func compressValue(value []byte) ([]byte, error) {
	var b strings.Builder
	gz := gzip.NewWriter(&b)
	_, err := gz.Write(value)
	if err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func decompressValue(value []byte) ([]byte, error) {
	gr, err := gzip.NewReader(strings.NewReader(string(value)))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	data, err := io.ReadAll(gr)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	var deleted int64
	for iter.Next(ctx) {
		key := iter.Val()
		n, err := c.client.Del(ctx, key).Result()
		if err != nil {
			return deleted, err
		}
		deleted += n
	}
	if err := iter.Err(); err != nil {
		return deleted, err
	}
	return deleted, nil
}

func (c *RedisCache) IncrAndGet(ctx context.Context, key string) (int64, error) {
	pipe := c.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	get := pipe.Get(ctx, key)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	return get.Val().(int64), nil
}

func (c *RedisCache) HExists(ctx context.Context, key, field string) (bool, error) {
	return c.client.HExists(ctx, key, field).Result()
}

func (c *RedisCache) BulkSetNX(ctx context.Context, keyValues map[string]interface{}, expiration time.Duration) (map[string]bool, error) {
	pipe := c.client.Pipeline()
	cmds := make(map[string]*redis.BoolCmd)

	for key, value := range keyValues {
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}

		compressedValue, err := compressValue(jsonValue)
		if err != nil {
			return nil, err
		}

		cmds[key] = pipe.SetNX(ctx, key, compressedValue, expiration)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	results := make(map[string]bool)
	for key, cmd := range cmds {
		results[key], err = cmd.Result()
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

func (c *RedisCache) GracefulShutdown(ctx context.Context) error {
	err := c.client.Close()
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return errors.New("shutdown timed out")
	case <-time.After(5 * time.Second):
		return nil
	}
}