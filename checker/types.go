package main

// ========== Constants ==========

const InstanceFile = "/data/licence_instance.json"

// ========== Global Structs ==========

type LicenseKey struct {
	ID              int    `json:"id"`
	Status          string `json:"status"`
	Key             string `json:"key"`
	ActivationLimit int    `json:"activation_limit"`
	ActivationUsage int    `json:"activation_usage"`
	CreatedAt       string `json:"created_at"`
	ExpiresAt       string `json:"expires_at"`
}

type Instance struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type Meta struct {
	StoreID       int    `json:"store_id"`
	OrderID       int    `json:"order_id"`
	OrderItemID   int    `json:"order_item_id"`
	ProductID     int    `json:"product_id"`
	ProductName   string `json:"product_name"`
	VariantID     int    `json:"variant_id"`
	VariantName   string `json:"variant_name"`
	CustomerID    int    `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
}

type ValidateResponse struct {
	Valid      bool       `json:"valid"`
	Error      string     `json:"error"`
	LicenseKey LicenseKey `json:"license_key"`
	Instance   *Instance  `json:"instance"`
	Meta       *Meta      `json:"meta"`
}

type ActivateResponse struct {
	Activated  bool       `json:"activated"`
	Error      string     `json:"error"`
	LicenseKey LicenseKey `json:"license_key"`
	Instance   Instance   `json:"instance"`
	Meta       *Meta      `json:"meta"`
}

type StoredInstance struct {
	EncryptedID string `json:"encrypted_id"`
}

type CheckRequest struct {
	LicenseKey   string `json:"license_key"`
	InstanceName string `json:"instance_name"`
}

type CheckResponse struct {
	Premium    bool   `json:"premium"`
	InstanceID string `json:"instance_id"`
	ExpiresAt  string `json:"expires_at"`
	Error      string `json:"error,omitempty"`
}
