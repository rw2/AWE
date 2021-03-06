package auth

import (
	"github.com/MG-RAST/AWE/lib/user"
	"sync"
	"time"
)

type cache struct {
	sync.Mutex
	m map[string]cacheValue
}

type cacheValue struct {
	expires time.Time
	user    *user.User
}

func (c *cache) lookup(header string) *user.User {
	if v, ok := c.m[header]; ok {
		if time.Now().Before(v.expires) {
			return v.user
		} else {
			c.Lock()
			defer c.Unlock()
			delete(c.m, header)
		}
	}
	return nil
}

func (c *cache) add(header string, u *user.User) {
	c.Lock()
	defer c.Unlock()
	c.m[header] = cacheValue{
		expires: time.Now().Add(1 * time.Hour),
		user:    u,
	}
	return
}
