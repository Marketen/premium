import React from "react";
import { useState } from "react";
import "./index.css";


export default function App() {
  const [licenseKey, setLicenseKey] = useState("");
  const [instanceName, setInstanceName] = useState("");
  const [result, setResult] = useState(null);

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
      setResult({
        ...data,
      });
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

  const handleValidateRandom = async () => {
    const randomId = self.crypto?.randomUUID?.() || generateRandomId();
    try {
      const res = await fetch("/api/check?force=true", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          license_key: licenseKey,
          instance_name: randomId,
        }),
      });
      const data = await res.json();
      setResult({
        ...data,
        tested_instance_id: randomId,
      });
    } catch (err) {
      setResult({
        error: "Request failed: " + err.message,
        tested_instance_id: randomId,
      });
    }
  };
  

  const generateRandomId = () => {
    return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, function (c) {
      const r = (Math.random() * 16) | 0;
      const v = c === "x" ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  };

  return (
    <main className="main">
      <div className="container">
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
          <button onClick={handleCheck}>Check License</button>
          <button className="red" onClick={handleDeactivate}>Deactivate</button>
          <button className="purple" onClick={handleValidateRandom}>Validate Random</button>
        </div>
        {result && (
          <pre>{JSON.stringify(result, null, 2)}</pre>
        )}
      </div>
    </main>
  );
}
