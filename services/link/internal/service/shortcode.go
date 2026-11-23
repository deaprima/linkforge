package service

import (
	"crypto/rand"
	"math/big"
)

const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const shortCodeLength = 6

func generateShortCode() (string, error) {
    result := make([]byte, shortCodeLength)
    for i := range result {
        n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
        if err != nil {
            return "", err
        }
        result[i] = charset[n.Int64()]
    }
    return string(result), nil
}