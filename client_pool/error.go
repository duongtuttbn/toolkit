package client_pool

import (
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"
)

func isLogTooLargeError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(rpc.Error); (ok && err.(rpc.Error).ErrorCode() == -32005 && strings.Contains(err.Error(), "10000")) || // rate limit error infura
		strings.Contains(err.Error(), "limit exceeded") || // rate limit getblockio
		strings.Contains(err.Error(), "range too large") { // rate limit cloudflare-eth.com
		return true
	}
	return false
}

func isRateLimit(err error) bool {
	if _, ok := err.(rpc.HTTPError); ok && (err.(rpc.HTTPError).StatusCode == http.StatusTooManyRequests || err.(rpc.HTTPError).StatusCode == -32429) ||
		strings.Contains(err.Error(), "Exceeded the quota usage") ||
		strings.Contains(err.Error(), "limit exceeded") ||
		strings.Contains(err.Error(), "exceeded limit") ||
		strings.Contains(err.Error(), "Unable to perform request") ||
		strings.Contains(err.Error(), "order a dedicated full node") {
		return true
	}
	return false
}
