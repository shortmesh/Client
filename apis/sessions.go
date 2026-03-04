package apis

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// Declare it globally using 'var'
var nonceCache *expirable.LRU[string, bool]

func SessionsCacheInit() {
	// Initialize it inside the special init() function or main()
	nonceCache = expirable.NewLRU[string, bool](10000, nil, time.Second*60)
}

func ValidateNonce(nonce string) bool {
	// 3. Logic remains the same
	if nonceCache.Contains(nonce) {
		return false
	}
	nonceCache.Add(nonce, true)
	return true
}
