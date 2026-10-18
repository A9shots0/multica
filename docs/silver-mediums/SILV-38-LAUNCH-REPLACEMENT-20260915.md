# SILV-38 one-time process-launch replacement

The fifth native task, `01a0a658-4e03-7ce3-b651-d2be994b0c87`, failed at
2026-09-15 18:33:30Z because Windows could not create the provider process:
"The paging file is too small for this operation to complete." Its safe receipt
contains no frames or result. The failed task, consumed slot, frozen harness,
operator plan and cleanup receipt remain intact.

The existing owner grant was clarified by coordination task
`01a06e85-9068-74f1-9c2d-64fc61dd7bd8`: replace this proven zero-provider-contact
launch exactly once, then perform the independent reviewer. The ceiling is seven
native attempts and six actual provider calls, with no additional retry. This is
not a claim that the fifth task succeeded or that its slot was unconsumed.

Before replacement, credential-free `--version` checks passed for both the
original executable and a fresh 230,228,640-byte copy, version 2.1.272. The copied
probe executable was removed. These checks establish process launch only, not
provider or workload success. Machine-wide memory settings and unrelated processes
remain unchanged. The production application still uses the reviewed exact images.

The copied operational harness changes only exact runtime/role/artifact paths and
the list of five preserved historical task identities. Provider transport, output
validation, credentials, model, tools, scope, cleanup and two-call enforcement are
unchanged. An explicit operator plan uses a newly captured read-only CRM snapshot.
Fresh authority binds the new exact binaries and role identities. The separate
writer and reviewer results will be recorded in the final outcome report.
