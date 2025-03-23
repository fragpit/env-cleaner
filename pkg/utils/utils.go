package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/xhit/go-str2duration/v2"
)

func SetDeleteAt(ttl string) (string, int64, error) {
	if ttl == "" {
		return "", 0, errors.New("ttl is empty")
	}

	ttlInt, err := str2duration.ParseDuration(ttl)
	if err != nil {
		return "", 0, fmt.Errorf("error parsing ttl: %w", err)
	}

	deleteAtTime := time.Now().Add(ttlInt)
	deleteAtStr := deleteAtTime.Format("02-01-06 15:04:05")

	return deleteAtStr, deleteAtTime.Unix(), nil
}

func IncreaseDeleteAt(date, period string) (string, int64, error) {
	increaseTime, err := str2duration.ParseDuration(period)
	if err != nil {
		return "", 0, fmt.Errorf("error parsing period: %w", err)
	}

	curTime, err := time.Parse("02-01-06 15:04:05", date)
	if err != nil {
		return "", 0, fmt.Errorf("error parsing date: %w", err)
	}

	newTime := curTime.Add(increaseTime)
	deleteAt := newTime.Format("02-01-06 15:04:05")
	deleteAtSec := newTime.Unix()

	return deleteAt, deleteAtSec, nil
}

// PeriodValidate validates that the provided period is less than the max period.
func PeriodValidate(period string, maxDuration string) error {
	maxD, err := str2duration.ParseDuration(maxDuration)
	if err != nil {
		return err
	}

	dur, err := str2duration.ParseDuration(period)
	if err != nil {
		return err
	}

	if dur > maxD {
		return errors.New("period is greater than max duration")
	}

	return nil
}

// GenerateToken generates a random token of length len.
func GenerateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
