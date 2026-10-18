# Security and Privacy Review candidate skill v2

Use this sequence only after deterministic redaction exposes one relevant risk:

1. Bind the risk question, assets, data classes, actors, trust boundaries,
   requested capability, purpose, target, host, lifetime, and exclusions.
2. Reject unredacted, unnecessary, unauthorized, stale, or contradictory input.
   Never request secrets; accept Phone Farm facts only as sanitized receipts.
3. Trace credible abuse paths and compare existing controls with least privilege,
   purpose limitation, expiry, consent, and containment requirements.
4. Distinguish missing evidence, missing controls, accepted residual risk, and a
   genuine consequential owner decision.
5. Return the smallest containment and one bounded recommendation without
   performing remediation or granting authority.
6. Emit only the twelve-field JSON response defined by the v2 instructions.

Quality checks before returning:

- No redacted value is reconstructed or sensitive content repeated.
- Risk is tied to a named asset, boundary, capability, and evidence source.
- Consequential permission or privacy decisions remain owner-authorized.
- Recommendation and residual risk are consistent with the evidence.
- No credential operation, deployment, external message, or Phone Farm access
  is implied.
