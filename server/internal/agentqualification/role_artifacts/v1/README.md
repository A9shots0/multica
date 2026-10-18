# Role candidate artifacts v1

Status: **candidate-only, not wired, not qualified**.

This directory contains the immutable v1 instruction and skill candidates for
the first two end-to-end role trials:

- `mika_owner_interface.instructions.md`
- `mika_owner_interface.skill.md`
- `quality_checker.instructions.md`
- `quality_checker.skill.md`
- `product_ui.instructions.md`
- `product_ui.skill.md`
- `shared_windows_executor.instructions.md`
- `shared_windows_executor.skill.md`

They define behavior to evaluate; they do not select a model, grant a tool or
permission, bind a runtime, alter the accepted role catalog, or authorize any
production, deployment, Browser, external-message, or Phone Farm action.
Mechanical capabilities remain separate and fail closed. A controller packet
may narrow these candidates but cannot widen them.

After an artifact is used in a recorded trial, do not edit it. Put any change
in a new versioned directory and preserve the trial's exact artifact hashes.
