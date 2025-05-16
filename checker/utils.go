package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// ========== Utilities ==========

func readInstance() StoredInstance {
	data, err := os.ReadFile(InstanceFile)
	if err != nil {
		return StoredInstance{}
	}
	var s StoredInstance
	_ = json.Unmarshal(data, &s)
	return s
}

func storeInstance(encryptedID string) error {
	data, err := json.Marshal(StoredInstance{EncryptedID: encryptedID})
	if err != nil {
		return err
	}
	return os.WriteFile(InstanceFile, data, 0644)
}

func loadDecryptedInstanceID() (string, error) {
	stored := readInstance()
	if stored.EncryptedID == "" {
		return "", errors.New("no stored instance to deactivate")
	}
	return decrypt(stored.EncryptedID)
}

func encrypt(value string) (string, error) {
	return value + "_encrypted", nil
}

func decrypt(value string) (string, error) {
	if strings.HasSuffix(value, "_encrypted") {
		return strings.TrimSuffix(value, "_encrypted"), nil
	}
	return "", errors.New("invalid encrypted string")
}

func nowISO8601() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000Z")
}

func writeJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}
