"use client";

import { create } from "zustand";

export type HqPageMode = "hq" | "classic";

interface HqState {
  mode: HqPageMode;
  selectedOutcomeId: string | null;
  setMode: (mode: HqPageMode) => void;
  selectOutcome: (issueId: string | null) => void;
}

export const useHqStore = create<HqState>((set) => ({
  mode: "hq",
  selectedOutcomeId: null,
  setMode: (mode) => set({ mode }),
  selectOutcome: (selectedOutcomeId) => set({ selectedOutcomeId }),
}));
