package token_pool

import (
	"chat2api/app/common"
	"sync"
)

var (
	instance *AccessTokenPool
	once     sync.Once
)

type AccessTokenPool struct {
	mu           sync.Mutex
	AccessTokens []*AccessToken
	index        int
}

type AccessToken struct {
	Token     string `yaml:"token,omitempty"`
	ExpiresAt int64  `yaml:"expires_at,omitempty"`
	Proxy     string `yaml:"proxy,omitempty"`
	CanUseAt  int64  `yaml:"-"`
}

func newAccessTokenPool() *AccessTokenPool {
	return &AccessTokenPool{
		AccessTokens: make([]*AccessToken, 0),
		index:        -1,
	}
}

func GetAccessTokenPool() *AccessTokenPool {
	once.Do(func() {
		instance = newAccessTokenPool()
	})
	return instance
}

func (a *AccessTokenPool) AddAccessToken(accessToken *AccessToken) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.AccessTokens = append(a.AccessTokens, accessToken)
}

func (a *AccessTokenPool) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.AccessTokens = make([]*AccessToken, 0)
	a.index = -1
}

func (a *AccessTokenPool) AppendAccessTokens(accessTokens []*AccessToken) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.AccessTokens = append(a.AccessTokens, accessTokens...)
}

func (a *AccessTokenPool) Size() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.AccessTokens)
}

func (a *AccessTokenPool) IsEmpty() bool {
	return a.Size() == 0
}

func (a *AccessTokenPool) CanUseSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := common.GetTimestampSecond(0)
	count := 0
	for _, v := range a.AccessTokens {
		if v.CanUseAt <= now && v.ExpiresAt > now {
			count++
		}
	}
	return count
}

func (a *AccessTokenPool) GetToken() string {
	accessToken := a.GetAccessToken()
	if accessToken == nil {
		return ""
	}
	return accessToken.Token
}

func (a *AccessTokenPool) GetAccessToken() *AccessToken {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.AccessTokens) == 0 {
		return nil
	}

	now := common.GetTimestampSecond(0)
	total := len(a.AccessTokens)

	for i := 0; i < total; i++ {
		a.index = (a.index + 1) % total
		token := a.AccessTokens[a.index]
		if token.CanUseAt <= now && token.ExpiresAt > now {
			return token
		}
	}

	return nil
}

func (a *AccessTokenPool) SetCanUseAt(token string, canUseAt int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, v := range a.AccessTokens {
		if v.Token == token {
			v.CanUseAt = canUseAt
			break
		}
	}
}
