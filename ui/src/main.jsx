import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

// Inject LemonSqueezy script once
const script = document.createElement("script");
script.src = "https://assets.lemonsqueezy.com/lemon.js";
script.defer = true;
document.body.appendChild(script);

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
