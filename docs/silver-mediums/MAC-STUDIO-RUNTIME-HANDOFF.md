# Mac Studio: joining Drew's Multica instance

Written 2026-09-17 from the Windows side. Nothing in here was run against Phone Farm, and nothing
here asks a Windows agent to touch it.

## Why this exists

Drew filed SILV-41, "Phone Farm: implement Schedule Cards build spec". Phone Farm work is Mac Studio
work only — the role catalog binds it to `mac_studio_only` and forbids every Windows role from it. His
Multica instance currently has four runtimes, all on the Windows machine, so there is no worker that
is allowed to take the issue. Until a Mac Studio runtime registers, the issue cannot start, and the
board now says exactly that rather than pretending otherwise.

## What the Mac Studio needs to do

The instance is reachable over Tailscale only. `silvers-mac-studio` is already on the tailnet, so no
new network access is required.

```bash
# 1. Confirm the instance is reachable from the Mac.
curl -s https://a9drew.tailda9924.ts.net/health

# 2. Install the CLI if it is not present (Homebrew tap, or the release binary).
multica version

# 3. Point the CLI at Drew's instance and sign in.
multica setup self-host \
  --server-url https://a9drew.tailda9924.ts.net \
  --app-url    https://a9drew.tailda9924.ts.net

# 4. Start the local runtime daemon.
multica daemon start
```

Sign-in uses an email code. **Drew's instance has no outbound email configured**, so the code never
arrives in an inbox — it is written to the backend log on the Windows host. Whoever is at the Mac
should say when they are at the code prompt, and the Windows side reads the code out of
`docker logs multica-backend-1`. Fixing email on the instance would remove this step and is worth
doing separately.

Once `multica runtime list` shows a Mac runtime as online, the two Phone Farm roles can be created
against it. They are deliberately absent today: creating them now would bind them to the Windows
runtime, which is exactly what the catalog forbids.

## The two roles, ready to create

From `server/internal/systemgraph/roles.json`, unchanged:

**Phone Farm Application Specialist** (`phonefarm_application_specialist`, primary executor,
persistent, `mac_studio_only`)
- May: read and change Phone Farm source, run the Mac tests, produce a candidate receipt.
- Never: run anything on Windows, approve its own work, change CRM authority, deploy without authority.
- Works by: all technical work stays on the Mac Studio; separate app behaviour from device health;
  return sanitised receipts.
- Independent from: Phone Fleet Reliability, Quality Checker.

**Phone Fleet Reliability** (`phone_fleet_reliability`, boundary contributor, persistent,
`mac_studio_only`)
- May: inspect device health, run bounded device checks, return fleet evidence.
- Never: run anything on Windows, edit application source, approve an application candidate, change
  CRM authority.
- Works by: do not diagnose app code without evidence; one named fleet boundary per contribution;
  return device-level results.
- Independent from: Phone Farm Application Specialist.

Both carry the `phone-farm-guide` skill. Once they exist, the Standard outcome delivery squad gains
them at its implementation and boundary-evidence stages, and SILV-41 can be bound to the application
specialist with the Quality Checker verifying.

## Command to create them, once a Mac runtime is online

```bash
multica agent create --name "Phone Farm Application Specialist" \
  --runtime-id <mac-runtime-id> --model claude-sonnet-5 \
  --instructions "$(cat role-phonefarm_application_specialist.md)"

multica agent create --name "Phone Fleet Reliability" \
  --runtime-id <mac-runtime-id> --model claude-sonnet-5 \
  --instructions "$(cat role-phone_fleet_reliability.md)"
```

The instruction files are generated the same way as the other nineteen roles: straight from the
catalog entry, with the house rules appended. The Windows side can produce both files and hand them
over; it must not create the agents, because agent creation requires naming the runtime they run on.

## What is still unknown

- Whether the Mac Studio has the Multica CLI installed, and at what version.
- Whether Phone Farm's own repo state on the Mac is ready for the Schedule Cards spec. That is a Mac
  question and stays there.
