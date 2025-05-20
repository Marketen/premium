import React from "react";
import { useEffect, useState } from "react";
import "./index.css";


export default function App() {
  const [licenseKey, setLicenseKey] = useState("");
  const [instanceName, setInstanceName] = useState("");
  const [result, setResult] = useState(null);
  const [licenseStored, setLicenseStored] = useState(false);


  // Move fetchStoredLicense outside useEffect so it can be reused
  const fetchStoredLicense = async () => {
    try {
      const res = await fetch("/api/license");
      if (res.ok) {
        const data = await res.json();
        setLicenseKey(data.key);
        setLicenseStored(true);
      } else {
        setLicenseStored(false);
      }
    } catch (err) {
      setLicenseStored(false);
    }
  };

  useEffect(() => {
    fetchStoredLicense();
  }, []);

  const handleActivate = async () => {
    setResult(null); // Clear previous result
    if (!licenseKey || !instanceName) {
      setResult({ error: "License key and instance name are required." });
      return;
    }
    try {
      const res = await fetch("/api/activate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          license_key: licenseKey,
          instance_name: instanceName,
        }),
      });
      const data = await res.json();
      setResult(data);
      if (!data.error) setLicenseStored(true);
    } catch (err) {
      setResult({ error: "Request failed: " + err.message });
    }
  };

  const handleCheck = async () => {
    try {
      const res = await fetch("/api/check", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          license_key: licenseKey,
          instance_name: instanceName,
        }),
      });
      const data = await res.json();
      setResult(data);
    } catch (err) {
      setResult({ error: "Request failed: " + err.message });
    }
  };
  

  const handleDeactivate = async () => {
    try {
      const res = await fetch("/api/deactivate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ license_key: licenseKey }),
      });
      const text = await res.text();
      setResult({ message: text });
    } catch (err) {
      setResult({ error: "Request failed: " + err.message });
    }
  };

  return (
    <main className="main">
      <div className="container">
        <a
        href="https://testdappnodepremium.lemonsqueezy.com/buy/91e0f96c-4e0e-4c2a-bf31-a6ebaef4fce5?embed=1"
        className="lemonsqueezy-button"
        >
        Buy Dappnode Premium
      </a>
        <h1>License Checker</h1>
        <input
          type="text"
          placeholder="License Key"
          value={licenseKey}
          onChange={(e) => setLicenseKey(e.target.value)}
        />
        <input
          type="text"
          placeholder="Instance Name"
          value={instanceName}
          onChange={(e) => setInstanceName(e.target.value)}
        />
        <div className="button-row">
          <button onClick={handleActivate}>Activate</button>
          <button onClick={handleCheck}>Check License</button>
          <button className="red" onClick={handleDeactivate}>Deactivate</button>
          <button className="purple" onClick={() => window.open('https://testdappnodepremium.lemonsqueezy.com/billing', '_blank')}>Manage Subscription</button>
        </div>
        {result && (
          <pre>{JSON.stringify(result, null, 2)}</pre>
        )}
      </div>
    </main>
  );
}
