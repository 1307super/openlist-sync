import { useState, useEffect, useCallback } from "react";
import { settingsApi } from "../api/client";
import type { Settings, TestConnectionResult } from "../types";

export function useSettings() {
  const [settings, setSettings] = useState<Settings>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testResult, setTestResult] = useState<TestConnectionResult | null>(null);
  const [testing, setTesting] = useState(false);

  const fetchSettings = useCallback(async () => {
    try {
      setLoading(true);
      const data = await settingsApi.get();
      setSettings(data);
    } catch (err) {
      console.error("Failed to fetch settings:", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchSettings();
  }, [fetchSettings]);

  const updateSettings = useCallback(async (data: Partial<Settings>) => {
    try {
      setSaving(true);
      await settingsApi.update(data);
      setSettings((prev) => ({ ...prev, ...data }));
    } finally {
      setSaving(false);
    }
  }, []);

  const testConnection = useCallback(async (data: Partial<Settings>) => {
    try {
      setTesting(true);
      setTestResult(null);
      // Save first so the backend has the latest credentials to test
      await settingsApi.update(data);
      const result = await settingsApi.test();
      setTestResult(result);
      setSettings((prev) => ({ ...prev, ...data }));
    } catch (err) {
      setTestResult({
        success: false,
        message: err instanceof Error ? err.message : "Test failed",
      });
    } finally {
      setTesting(false);
    }
  }, []);

  return {
    settings,
    loading,
    saving,
    testResult,
    testing,
    updateSettings,
    testConnection,
  };
}
