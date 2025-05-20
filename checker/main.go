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

// ====== Server Entrypoint ======

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/api/check", checkLicenseHandler).Methods("POST")
	router.HandleFunc("/api/deactivate", deactivateHandler).Methods("POST")
	router.HandleFunc("/api/license", getLicenseKeyHandler).Methods("GET")

	log.Printf("Server started on %s", Port)
	if err := http.ListenAndServe(Port, router); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// ====== HTTP Handlers ======

func checkLicenseHandler(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPError(w, "Invalid request", http.StatusBadRequest)
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
		writeHTTPError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	instanceID, err := loadDecryptedInstanceID()
	if err != nil {
		writeHTTPError(w, err.Error(), http.StatusBadRequest)
		return
	}

	form := fmt.Sprintf("license_key=%s&instance_id=%s", req.LicenseKey, instanceID)
	reqDeactivate, _ := http.NewRequest("POST", "https://api.lemonsqueezy.com/v1/licenses/deactivate", strings.NewReader(form))
	reqDeactivate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqDeactivate.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(reqDeactivate)
	if err != nil {
		writeHTTPError(w, "Failed to deactivate license", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	if _, err := readAndLogResponseBody(res, "deactivate"); err != nil {
		writeHTTPError(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	var result struct {
		Deactivated bool   `json:"deactivated"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		writeHTTPError(w, "Invalid response from API", http.StatusInternalServerError)
		return
	}

	if !result.Deactivated {
		writeHTTPError(w, fmt.Sprintf("Failed to deactivate: %s", result.Error), http.StatusBadRequest)
		return
	}

	// Remove both stored instance and license key file
	_ = os.Remove(InstanceFile)
	_ = os.Remove(LicenseFilePath)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deactivated successfully"))
}

func getLicenseKeyHandler(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open(LicenseFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			writeHTTPError(w, "License not found", http.StatusNotFound)
		} else {
			writeHTTPError(w, "Error reading license file", http.StatusInternalServerError)
		}
		return
	}
	defer file.Close()

	var data struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		writeHTTPError(w, "Invalid license file format", http.StatusInternalServerError)
		return
	}
	writeJSON(w, data)
}

// ====== Core License Logic ======

// validateOrActivateLicense validates the license or activates it if needed.
func validateOrActivateLicense(key, instanceName string, force bool) (premium bool, instanceID, expiresAt string, err error) {
	if force {
		return validateLicense(key, "invalid-forced-instance-id")
	}
	stored := readInstance()
	if stored.EncryptedID != "" {
		if id, decErr := decrypt(stored.EncryptedID); decErr == nil {
			return validateLicense(key, id)
		} else {
			log.Println("Decryption failed, activating instead:", decErr)
		}
	} else {
		log.Println("No stored instance ID, activating instead")
	}
	return activateLicense(key, instanceName)
}

// activateLicense activates a license and stores the instance and license key.
func activateLicense(key, instanceName string) (premium bool, instanceID, expiresAt string, err error) {
	// Check if license file already exists
	if _, statErr := os.Stat(LicenseFilePath); statErr == nil {
		return false, "", "", fmt.Errorf("license already activated: %s exists", LicenseFilePath)
	} else if !os.IsNotExist(statErr) {
		return false, "", "", fmt.Errorf("could not check license file: %w", statErr)
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

	// Encrypt and store instance ID
	if encryptedID, encErr := encrypt(result.Instance.ID); encErr == nil {
		_ = storeInstance(encryptedID)
	}

	// Save only the license key to file
	licenseKeyJSON := struct {
		Key string `json:"key"`
	}{Key: result.LicenseKey.Key}

	file, err := os.OpenFile(LicenseFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return false, "", "", fmt.Errorf("failed to create license file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(licenseKeyJSON); err != nil {
		return false, "", "", fmt.Errorf("failed to write license file: %w", err)
	}

	valid := result.LicenseKey.Status == "active" &&
		(result.LicenseKey.ExpiresAt == "" || result.LicenseKey.ExpiresAt > nowISO8601())

	return valid, result.Instance.ID, result.LicenseKey.ExpiresAt, nil
}

// validateLicense validates a license with the given key and instance ID.
func validateLicense(key, instanceID string) (premium bool, instanceIDResult, expiresAt string, err error) {
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

	if result.Instance != nil {
		instanceIDResult = result.Instance.ID
	}
	return valid, instanceIDResult, result.LicenseKey.ExpiresAt, nil
}

// ====== Helper Functions ======

// writeHTTPError writes a JSON error response with the given status code.
func writeHTTPError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
