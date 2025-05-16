package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

// ========== Server Entrypoint ==========

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/api/check", checkLicenseHandler).Methods("POST")
	router.HandleFunc("/api/deactivate", deactivateHandler).Methods("POST")

	const port = ":8060"
	log.Printf("Server started on %s", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// ========== Handlers ==========

func checkLicenseHandler(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	premium, instanceID, expiresAt, err := validateOrActivateLicense(req.LicenseKey, req.InstanceName, force)

	resp := CheckResponse{
		Premium:    premium,
		InstanceID: instanceID,
		ExpiresAt:  expiresAt,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	writeJSON(w, resp)
}

func deactivateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicenseKey string `json:"license_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	instanceID, err := loadDecryptedInstanceID()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

	if _, err := readAndLogResponseBody(res, "deactivate"); err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

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

	_ = os.Remove(InstanceFile)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deactivated successfully"))
}

// ========== Core License Logic ==========

func validateOrActivateLicense(key, instanceName string, force bool) (bool, string, string, error) {
	if force {
		return validateLicense(key, "invalid-forced-instance-id")
	}

	if stored := readInstance(); stored.EncryptedID != "" {
		if id, err := decrypt(stored.EncryptedID); err == nil {
			return validateLicense(key, id)
		} else {
			log.Println("Decryption failed, activating instead:", err)
		}
	} else {
		log.Println("No stored instance ID, activating instead")
	}

	return activateLicense(key, instanceName)
}

func activateLicense(key, instanceName string) (bool, string, string, error) {
	form := fmt.Sprintf("license_key=%s&instance_name=%s", key, instanceName)
	req, _ := http.NewRequest("POST", "https://api.lemonsqueezy.com/v1/licenses/activate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", "", fmt.Errorf("activation failed: %w", err)
	}
	defer res.Body.Close()

	if _, err := readAndLogResponseBody(res, "activate"); err != nil {
		return false, "", "", fmt.Errorf("failed reading response: %w", err)
	}

	var result ActivateResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return false, "", "", fmt.Errorf("invalid response: %w", err)
	}

	if res.StatusCode >= 400 {
		return false, "", "", fmt.Errorf("activation failed (%d): %s", res.StatusCode, result.Error)
	}

	if encryptedID, err := encrypt(result.Instance.ID); err == nil {
		_ = storeInstance(encryptedID)
	}

	valid := result.LicenseKey.Status == "active" &&
		(result.LicenseKey.ExpiresAt == "" || result.LicenseKey.ExpiresAt > nowISO8601())

	return valid, result.Instance.ID, result.LicenseKey.ExpiresAt, nil
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

	if _, err := readAndLogResponseBody(res, "validate"); err != nil {
		return false, "", "", fmt.Errorf("failed reading response: %w", err)
	}

	var result ValidateResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return false, "", "", fmt.Errorf("invalid response: %w", err)
	}

	if res.StatusCode >= 400 || !result.Valid {
		return false, "", "", fmt.Errorf("license not valid: %s", result.Error)
	}

	valid := result.LicenseKey.Status == "active" &&
		(result.LicenseKey.ExpiresAt == "" || result.LicenseKey.ExpiresAt > nowISO8601())

	instanceIDResult := ""
	if result.Instance != nil {
		instanceIDResult = result.Instance.ID
	}
	return valid, instanceIDResult, result.LicenseKey.ExpiresAt, nil
}
