import { useEffect, useState } from "react";

export default function App() {
  const [health, setHealth] = useState<string>("checking...");

  useEffect(() => {
    fetch("/api/health")
      .then((r) => r.json())
      .then((d) => setHealth(d.status))
      .catch(() => setHealth("error"));
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <h1 className="text-2xl font-bold text-gray-800">AI Agent Tut</h1>
        <p className="mt-2 text-gray-600">
          API health: <span className="font-mono">{health}</span>
        </p>
        <p className="mt-1 text-sm text-gray-400">
          Mốc 0 — nền móng. Xem docs/plans để đi tiếp.
        </p>
      </div>
    </div>
  );
}
