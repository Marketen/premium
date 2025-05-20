package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ====== Utilities ======

// readInstance loads the stored instance from file, or returns an empty struct if not found.
func readInstance() StoredInstance {
	data, err := os.ReadFile(InstanceFile)
	if err != nil {
		return StoredInstance{}
	}
	var s StoredInstance
	_ = json.Unmarshal(data, &s)
	return s
}

// storeInstance saves the encrypted instance ID to file.
func storeInstance(encryptedID string) error {
	data, err := json.Marshal(StoredInstance{EncryptedID: encryptedID})
	if err != nil {
		return err
	}
	return os.WriteFile(InstanceFile, data, InstanceFilePerm)
}

// loadDecryptedInstanceID returns the decrypted instance ID or an error if not found.
func loadDecryptedInstanceID() (string, error) {
	stored := readInstance()
	if stored.EncryptedID == "" {
		return "", errors.New("no stored instance to deactivate")
	}
	return decrypt(stored.EncryptedID)
}

// encrypt simulates encryption for the instance ID.
func encrypt(value string) (string, error) {
	return value + "_encrypted", nil
}

// decrypt simulates decryption for the instance ID.
func decrypt(value string) (string, error) {
	if strings.HasSuffix(value, "_encrypted") {
		return strings.TrimSuffix(value, "_encrypted"), nil
	}
	return "", errors.New("invalid encrypted string")
}

// nowISO8601 returns the current UTC time in ISO8601 format.
func nowISO8601() string {
	return time.Now().UTC().Format(TimeFormat)
}

// writeJSON writes a JSON response with the correct content type.
func writeJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// readAndLogResponseBody reads and logs the HTTP response body, then resets it for further reading.
func readAndLogResponseBody(res *http.Response, label string) ([]byte, error) {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	log.Printf("Lemon API [%s] Response (%d): %s\n", label, res.StatusCode, string(body))

	// Reset body so it can be parsed again
	res.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}
