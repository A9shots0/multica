"use client";
import { useT } from "../../i18n";

import { useState } from "react";
import { CheckCircle2, ShieldCheck, Rocket, GitCommit, Clock } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";

interface DeploymentCandidate {
  id: string;
  service: string;
  target: string;
  revision: string;
  branch: string;
  commitMsg: string;
  qcStatus: "passed" | "pending" | "failed";
  qcSummary: string;
  testedBy: string;
  stage: "staging" | "production";
}

const CANDIDATES: DeploymentCandidate[] = [
  {
    id: "dep-0a1a5c8",
    service: "Multica Control Plane & Backend",
    target: "Production (Linux / Docker)",
    revision: "4475751ea",
    branch: "main",
    commitMsg: "fix: chat owner intake direct worker queue and board priority accents",
    qcStatus: "passed",
    qcSummary: "Independent review passed · 11 unit & regression tests green · DB migration verified",
    testedBy: "QC Worker (Verified)",
    stage: "production",
  },
  {
    id: "dep-phone-farm",
    service: "Phone Farm Sidecar & Automation",
    target: "Mac Studio (M2 Max / arm64)",
    revision: "fd2f0ce62",
    branch: "release/phone-farm-20260920",
    commitMsg: "feat: phone farm account status automation & Kaya review visibility",
    qcStatus: "passed",
    qcSummary: "Pre-deployment checks green · Clean git tree verified · Rollback rehearsal passed",
    testedBy: "Mac Specialist QC",
    stage: "staging",
  },
];

export function DeploymentsTab() {
  const { t: tc } = useT("common");
  const [deployingId, setDeployingId] = useState<string | null>(null);
  const [deployed, setDeployed] = useState<Record<string, boolean>>({
    "dep-0a1a5c8": true,
  });

  const handleApprove = (candidate: DeploymentCandidate) => {
    setDeployingId(candidate.id);
    setTimeout(() => {
      setDeployingId(null);
      setDeployed((prev) => ({ ...prev, [candidate.id]: true }));
    }, 1200);
  };

  return (
    <div className="space-y-6">
      {/* Header and manual release gate explanation */}
      <div>
        <h2 className="text-title font-medium text-foreground">{tc(($) => $.operational_copy.deployment_approvals)}</h2>
        <p className="text-caption text-muted-foreground mt-1">{tc(($) => $.operational_copy.deployment_approvals_description)}</p>
      </div>

      <div className="flex items-start gap-3 rounded-lg border border-primary/20 bg-primary/5 p-4 text-body">
        <ShieldCheck className="size-5 shrink-0 text-primary mt-0.5" />
        <div className="space-y-1">
          <div className="flex items-center gap-2 font-medium text-foreground">
            <span>{tc(($) => $.operational_copy.owner_authorization_gate_active)}</span>
            <Badge variant="outline" className="border-primary/40 text-primary bg-primary/10 text-micro">{tc(($) => $.operational_copy.strict_gate)}</Badge>
          </div>
          <p className="text-caption text-muted-foreground leading-relaxed">{tc(($) => $.operational_copy.deployment_gate_explanation)}{" "}<strong>{tc(($) => $.operational_copy.approve_deploy)}</strong>.
          </p>
        </div>
      </div>

      {/* Deployment candidate cards */}
      <div className="space-y-4">
        {CANDIDATES.map((candidate) => {
          const isDeployed = deployed[candidate.id];
          const isBusy = deployingId === candidate.id;

          return (
            <div
              key={candidate.id}
              className="rounded-lg border bg-card p-5 shadow-sm space-y-4 transition-all"
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-medium text-foreground text-body">{candidate.service}</span>
                    <Badge variant="secondary" className="text-micro font-normal">
                      {candidate.target}
                    </Badge>
                    <Badge
                      variant="outline"
                      className={`text-micro capitalize ${
                        candidate.stage === "production"
                          ? "border-destructive/40 text-destructive bg-destructive/5"
                          : "border-warning/40 text-warning bg-warning/5"
                      }`}
                    >
                      {candidate.stage}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2 text-caption text-muted-foreground">
                    <GitCommit className="size-3.5" />
                    <span className="font-mono text-foreground font-medium">{candidate.revision}</span>
                    <span>{tc(($) => $.operational_copy.on)}</span>
                    <span className="font-mono">{candidate.branch}</span>
                    <span>—</span>
                    <span className="truncate max-w-[280px] sm:max-w-md">{candidate.commitMsg}</span>
                  </div>
                </div>

                <div>
                  {isDeployed ? (
                    <Badge variant="outline" className="gap-1.5 py-1.5 px-3 border-success/40 text-success bg-success/10 font-medium">
                      <CheckCircle2 className="size-4" />{" "}{tc(($) => $.operational_copy.live_deployed)}</Badge>
                  ) : (
                    <Button
                      size="sm"
                      className="gap-2 bg-primary hover:bg-primary/90 text-primary-foreground font-medium shadow-sm"
                      disabled={isBusy}
                      onClick={() => handleApprove(candidate)}
                    >
                      {isBusy ? (
                        <>
                          <Clock className="size-4 animate-spin" />{" "}{tc(($) => $.operational_copy.deploying)}</>
                      ) : (
                        <>
                          <Rocket className="size-4" />{" "}{tc(($) => $.operational_copy.approve_deploy)}</>
                      )}
                    </Button>
                  )}
                </div>
              </div>

              {/* QC / Evidence Verification box */}
              <div className="rounded-md border border-muted bg-muted/40 p-3 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-caption">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="size-4 text-success shrink-0" />
                  <span className="text-muted-foreground">{candidate.qcSummary}</span>
                </div>
                <div className="shrink-0 font-medium text-foreground">{tc(($) => $.operational_copy.verified_by)}{" "}{candidate.testedBy}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
