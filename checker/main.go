package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const instanceFile = "/data/license_instance.json"

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/api/check", checkLicenseHandler).Methods("POST")
	r.HandleFunc("/api/deactivate", deactivateHandler).Methods("POST")

	port := ":8060"
	log.Printf("Server started on %s", port)
	http.ListenAndServe(port, r)
}

type CheckRequest struct {
	LicenseKey   string `json:"license_key"`
	InstanceName string `json:"instance_name"`
}

type CheckResponse struct {
	Premium    bool   `json:"premium"`
	Error      string `json:"error,omitempty"`
	InstanceID string `json:"instance_id,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

type storedInstance struct {
	EncryptedID string `json:"encrypted_instance_id"`
}

func checkLicenseHandler(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "true"

	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	premium, instanceID, expiresAt, err := validateOrActivateLicense(req.LicenseKey, req.InstanceName, force)
	resp := CheckResponse{
		Premium:    premium,
		InstanceID: instanceID,
		ExpiresAt:  expiresAt,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func deactivateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseKey string `json:"license_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	stored := readInstance()
	if stored.EncryptedID == "" {
		http.Error(w, "No stored instance to deactivate", http.StatusBadRequest)
		return
	}
	instanceID, err := decrypt(stored.EncryptedID)
	if err != nil {
		http.Error(w, "Failed to decrypt instance ID", http.StatusInternalServerError)
		return
	}

	form := fmt.Sprintf("license_key=%s&instance_id=%s", req.LicenseKey, instanceID)
	reqDeactivate, _ := http.NewRequest("POST", "https://api.lemonsqueezy.com/v1/licenses/deactivate", strings.NewReader(form))
	reqDeactivate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqDeactivate.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(reqDeactivate)
	if err != nil {
		http.Error(w, "Failed to deactivate license", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	var result struct {
		Deactivated bool   `json:"deactivated"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid response from API", http.StatusInternalServerError)
		return
	}

	if !result.Deactivated {
		http.Error(w, fmt.Sprintf("Failed to deactivate: %s", result.Error), http.StatusBadRequest)
		return
	}

	_ = os.Remove(instanceFile)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deactivated successfully"))
}

func validateOrActivateLicense(key, instanceName string, force bool) (bool, string, string, error) {
	if !force {
		stored := readInstance()
		if stored.EncryptedID != "" {
			if instanceID, err := decrypt(stored.EncryptedID); err == nil {
				return validateLicense(key, instanceID)
			}
		}
	}

	form := fmt.Sprintf("license_key=%s&instance_name=%s", key, instanceName)
	req, _ := http.NewRequest("POST", "https://api.lemonsqueezy.com/v1/licenses/activate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", "", fmt.Errorf("activation failed: %w", err)
	}
	defer res.Body.Close()

	var result struct {
		Activated  bool   `json:"activated"`
		Error      string `json:"error"`
		LicenseKey struct {
			Status    string `json:"status"`
			ExpiresAt string `json:"expires_at"`
		} `json:"license_key"`
		Instance struct {
			ID string `json:"id"`
		} `json:"instance"`
	}

	if res.StatusCode == 200 {
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			return false, "", "", fmt.Errorf("invalid response: %w", err)
		}
		if encryptedID, err := encrypt(result.Instance.ID); err == nil {
			_ = storeInstance(encryptedID)
		}
		valid := result.LicenseKey.Status == "active" && (result.LicenseKey.ExpiresAt == "" || result.LicenseKey.ExpiresAt > nowISO8601())
		return valid, result.Instance.ID, result.LicenseKey.ExpiresAt, nil
	}

	// return the error returned by the API if the status code is not 200
	if res.StatusCode >= 400 {
		var errorResponse struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(res.Body).Decode(&errorResponse); err != nil {
			return false, "", "", fmt.Errorf("invalid response: %w", err)
		}
		return false, "", "", fmt.Errorf("activation failed: %s", errorResponse.Error)
	}
	// If the status code is not 200 and no error message is returned, return a generic error
	return false, "", "", fmt.Errorf("activation failed: unexpected status code %d", res.StatusCode)
}

func validateLicense(key, instanceID string) (bool, string, string, error) {
	form := fmt.Sprintf("license_key=%s&instance_id=%s", key, instanceID)
	req, _ := http.NewRequest("POST", "https://api.lemonsqueezy.com/v1/licenses/validate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", "", fmt.Errorf("validation failed: %w", err)
	}
	defer res.Body.Close()

	var result struct {
		Valid      bool   `json:"valid"`
		Error      string `json:"error"`
		LicenseKey struct {
			Status    string `json:"status"`
			ExpiresAt string `json:"expires_at"`
		} `json:"license_key"`
		Instance struct {
			ID string `json:"id"`
		} `json:"instance"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return false, "", "", fmt.Errorf("invalid response: %w", err)
	}

	if !result.Valid {
		return false, "", "", fmt.Errorf("license not valid: %s", result.Error)
	}

	valid := result.LicenseKey.Status == "active" && (result.LicenseKey.ExpiresAt == "" || result.LicenseKey.ExpiresAt > nowISO8601())
	return valid, result.Instance.ID, result.LicenseKey.ExpiresAt, nil
}

func storeInstance(encryptedID string) error {
	data := storedInstance{EncryptedID: encryptedID}
	f, err := os.Create(instanceFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(data)
}

func readInstance() storedInstance {
	f, err := os.Open(instanceFile)
	if err != nil {
		return storedInstance{}
	}
	defer f.Close()
	var data storedInstance
	_ = json.NewDecoder(f).Decode(&data)
	return data
}

func nowISO8601() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func encrypt(plainText string) (string, error) {
	key := []byte(os.Getenv("HOSTNAME"))
	if len(key) < 32 {
		key = append(key, make([]byte, 32-len(key))...)
	} else if len(key) > 32 {
		key = key[:32]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func decrypt(encoded string) (string, error) {
	key := []byte(os.Getenv("HOSTNAME"))
	if len(key) < 32 {
		key = append(key, make([]byte, 32-len(key))...)
	} else if len(key) > 32 {
		key = key[:32]
	}
	cipherText, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, cipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}
