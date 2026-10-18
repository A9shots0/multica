# Item 14 role-stage evidence contract v1

- Status: accepted provider-free candidate contract
- Machine contract:
  `server/internal/agentqualification/role_stage_evidence_contract_v1.json`
- Contract SHA-256:
  `f0105e916d826407b499d0de76c39e11fe9c29c96570fa04c5ef18f49ed705f5`

## Purpose

The primary-subscription trial showed that a specialist can produce valid
advice while the complete route still fails at the executor boundary. This
additive contract makes every role response and controller handoff mechanical
without changing the accepted role artifacts, choosing a model, or creating a
seat.

It binds all 11 staged role packets, 20 routes, 59 ordered participant stages,
and 10 shared-skill attachments to their exact artifact bindings and top-level
JSON fields. Any missing, extra, reordered, reclassified, or hash-drifted role
contract fails closed.

## Handoff rule

Roles never hand work directly to each other. Every stage returns its exact
evidence class to the deterministic controller. The controller reissues the
complete original packet for the next stage and may attach prior specialist
evidence as advisory context only. That evidence cannot expand authority,
replace required checks, or let the downstream executor return a partial
payload.

For tool-free controller-mediated execution, the shared executor must return a
self-contained action envelope. Each action declares all six fields. An edit's
`contents` value must be the complete literal file contents—not a description,
diff, instruction, or placeholder. Checks must be exact allowlist members;
Browser remains controller-owned local loopback; `finish` appears once at the
end. A failed or invalid stage is terminal without automatic repair or replay.

## Evidence and authority boundary

The contract preserves the external Mac-only Phone Farm evidence classes and
does not inspect or operate Phone Farm from Windows. Provider calls,
credentials, repository access, production actions, Windows Phone Farm access,
model selection, and permanent-seat selection are forbidden in this slice.

Focused tests resolve every response example from the immutable instruction
artifact, require each paired skill to retain its exact JSON-output boundary,
and reject provider/selection/authority expansion, direct handoff, incomplete
controller packets, nonliteral edits, automatic replay, missing fields,
Phone Farm reclassification, or altered trial findings.

## Integrated rehearsal result

The contract was replayed through both the 20-route all-role rehearsal and the
complete four-workflow local shadow suite. The recorded result contains 91
receipts: 69 all-route receipts and 22 local-shadow receipts. It binds 89
role-response contracts, preserves two controller-only outcome bindings,
reissues 79 complete controller packets, retains 10 in-stage shared-skill
attachments, and applies the self-contained action obligation to all 20 shared
Windows executor stages.

The 11 role schemas expose 112 role-qualified top-level fields. The two failed
Product/UI handoffs from the subscription trial remain explicit source
evidence rather than being reclassified. Mutation tests reject trace, response
class, packet, advisory-evidence, executor-action, shared-skill, controller-only,
provider, or selection drift. Provider, credential, tool, repository,
production, outbound, Phone Farm, and selection counts are zero.

Combined trace SHA-256:
`7cad59a9b3beef094ba47846f8c5b4adb47d2d1863dd43ec895ee5cd90690054`.
Recorded result SHA-256:
`45c7294b5a15b36a9bd702fc3241aaeaf2697282897d99eb0b362e9a38d02217`.
