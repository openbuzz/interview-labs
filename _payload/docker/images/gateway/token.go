package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

const cookieName = "gw_auth"

// itoa is a tiny alias so tests and code share one integer formatting path.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// signToken returns "<exp>.<base64url(HMAC_SHA256(secret, exp))>".
func signToken(secret []byte, exp int64) string {
	expStr := itoa(exp)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(expStr))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return expStr + "." + sig
}

// validToken recomputes the HMAC in constant time and requires exp in the future.
func validToken(secret []byte, value string, now time.Time) bool {
	dot := strings.LastIndexByte(value, '.')
	if dot <= 0 || dot == len(value)-1 {
		return false
	}

	expStr, sig := value[:dot], value[dot+1:]
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(expStr))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(want)) != 1 {
		return false
	}

	return exp > now.Unix()
}
