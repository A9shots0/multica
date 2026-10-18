# Item 14 provisional role configuration v1

- Status: accepted reversible provider-free local scope
- Runtime scope: frozen synthetic R01 fixtures only
- Provider, model, squad, permanent-role, and production selection: none

## Decision

The reliability v2 result supports one narrow topology decision: the shared
Windows executor may perform R01-like local fixture work when the exact
architecture-assurance skill packet is attached inline. This is a topology
binding only. It does not choose Claude, Codex, a provider, a model, a squad,
or a permanent seat.

The binding is
`server/internal/agentqualification/provisional_role_configuration_v1.json`
(SHA-256
`017408d91dbe84c9bb8c966b179ff2e66bb390115865905da732b2188f3ad991`).
It is reversible, cannot dispatch automatically, and allows only deterministic
provider-free execution against the frozen local R01 fixture.

## Rehearsed boundary

The deterministic rehearsal contains 20 scenarios. The exact R01 participant,
inline skill, artifact bindings, local-fixture mode, and unselected provider,
model, and squad combination is the sole allow. Nineteen mutations deny R02,
Product/UI, a separate specialist, provider/model/squad selection, a provider
call, credential access, native tools, repository access, production, outbound
actions, Phone Farm, retry, fallback, permanence, and automatic dispatch.

The rehearsal result is
`server/internal/agentqualification/provisional_role_configuration_rehearsal_v1_result.json`
(SHA-256
`2e84fb129e5ddac47762511d29ff02b7ee5ec582a35f092e2df23f38ebbd9176`).
Its trace SHA-256 is
`b89baa151438144fd1bc5bebd34f10829112ffa68dc3f1f2d4315fcc4c8c9f66`.

## Complete structural readiness

The v2 readiness matrix keeps every Item 14 artifact visible: 11 role packets,
11 instruction/skill pairs, ten controller-only outcome identities, one
Mac-only skill packet, and three squads. All are structurally bound. Only the
shared Windows executor is runtime-provisional, and only for R01 with the
inline architecture skill. No squad is activated; every other role remains
structural-only or explicitly held.

The matrix is
`server/internal/agentqualification/role_squad_readiness_matrix_v2.json`
(SHA-256
`4116a6d8b0b026d313dfe205d4cb62f8b7c0cf63a9532e2ba98b704483f43c8f`).

## Evidence boundary

This provider-free binding made zero provider calls, credential reads, native
tool executions, repository accesses, production accesses, outbound actions,
Phone Farm accesses, model selections, permanent-role selections, or squad
activations. It is local qualification evidence only, not production or
business acceptance.
