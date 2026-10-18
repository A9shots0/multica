// Synthetic owner-review acceptance fixture for the isolated local candidate.
// No task, runtime, model, provider, or production resource is created.
import { createHash, randomUUID } from "node:crypto";
import pg from "pg";
import sharp from "sharp";
import { ownerReviewConversationScenario as scenario } from "./owner-review-conversation-scenario.mjs";

const project = "smk-local-candidate-20260915";
const webBase = "http://127.0.0.1:13004";
const apiBase = "http://127.0.0.1:18084";
const email = "preview-owner@local.invalid";
const workspaceSlug = "local-candidate-samples";
const sessionTitle = scenario.sessionTitle;

function stableFixtureUUID(scope) {
  const hex = createHash("sha256").update(`owner-review-conversation:${scope}`).digest("hex");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20, 32)}`;
}

const { execFileSync } = await import("node:child_process");
const gateway = execFileSync(
  "docker",
  [
    "ps",
    "-q",
    "--filter",
    `label=com.docker.compose.project=${project}`,
    "--filter",
    "label=com.docker.compose.service=gateway",
  ],
  { encoding: "utf8" },
).trim();
if (!/^[a-f0-9]{12,64}$/.test(gateway)) {
  throw new Error("Expected exactly one isolated local-candidate gateway.");
}

let token = "";
async function api(path, body) {
  const response = await fetch(apiBase + path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
  });
  if (!response.ok) throw new Error(`Local fixture API ${path}: HTTP ${response.status}`);
  return response.json();
}

// A prior signed-in Browser pass may have just requested this same local-only
// code. Preserve the login throttle instead of weakening it for a fixture;
// 429 here only means the existing development code is still the active one.
const sendCode = await fetch(apiBase + "/auth/send-code", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email }),
});
if (!sendCode.ok && sendCode.status !== 429) {
  throw new Error(`Local fixture API /auth/send-code: HTTP ${sendCode.status}`);
}
const login = await api("/auth/verify-code", { email, code: "654321" });
token = login.token;

const client = new pg.Client({
  host: "127.0.0.1",
  port: 15437,
  database: "multica_preview",
  user: "multica",
  password: "local-preview-only",
});
await client.connect();

let workspace;
let owner;
const issues = new Map();
let agent;
let session;
try {
  workspace = (
    await client.query("SELECT id, issue_counter FROM workspace WHERE slug=$1", [workspaceSlug])
  ).rows[0];
  if (!workspace) throw new Error("The base local-candidate fixture has not been initialized.");
  owner = (
    await client.query(
      `SELECT u.id
       FROM "user" u
       JOIN member m ON m.user_id=u.id AND m.workspace_id=$1 AND m.role='owner'
       WHERE u.email=$2`,
      [workspace.id, email],
    )
  ).rows[0];
  if (!owner) throw new Error("Synthetic preview owner is missing.");

  await client.query("BEGIN");
  agent = (
    await client.query(
      `INSERT INTO agent
         (id,workspace_id,name,runtime_mode,runtime_config,visibility,status,owner_id,
          description,instructions,permission_mode,kind,system_key)
       VALUES ($1,$2,'Preview Mika','local','{}','private','offline',$3,
         'Mika is ready to walk through the result with you.',
         'Display fixture only. Do not dispatch.', 'private','user','mika')
       ON CONFLICT (workspace_id,name) DO UPDATE
         SET kind='user', system_key='mika',
             description='Mika is ready to walk through the result with you.', updated_at=now()
       RETURNING id`,
      [randomUUID(), workspace.id, owner.id],
    )
  ).rows[0];

  for (const issueSpec of scenario.issues) {
    let issue = (
      await client.query(
        "SELECT id,number FROM issue WHERE workspace_id=$1 AND title = ANY($2::text[]) ORDER BY updated_at DESC LIMIT 1",
        [workspace.id, [issueSpec.title, ...issueSpec.legacyTitles]],
      )
    ).rows[0];
    if (!issue) {
      const counter = (
        await client.query(
          "UPDATE workspace SET issue_counter=issue_counter+1 WHERE id=$1 RETURNING issue_counter",
          [workspace.id],
        )
      ).rows[0];
      const number = Number(counter.issue_counter);
      issue = (
        await client.query(
          `INSERT INTO issue
             (id,workspace_id,title,description,status,priority,creator_type,creator_id,
              number,position,last_activity_at)
           VALUES ($1,$2,$3,$4,'in_review','none','member',$5,$6,$7,now())
           RETURNING id,number`,
          [randomUUID(), workspace.id, issueSpec.title, issueSpec.description, owner.id, number, number],
        )
      ).rows[0];
    }
    await client.query(
      "UPDATE issue SET title=$1, description=$2, status='in_review', updated_at=now() WHERE id=$3",
      [issueSpec.title, issueSpec.description, issue.id],
    );
    issues.set(issueSpec.key, { ...issue, ...issueSpec });
  }

  session = (
    await client.query(
      `SELECT id FROM chat_session
       WHERE workspace_id=$1 AND creator_id=$2 AND title = ANY($3::text[])
       ORDER BY created_at DESC LIMIT 1`,
      [workspace.id, owner.id, [sessionTitle, "Owner review acceptance sample"]],
    )
  ).rows[0];
  if (!session) {
    session = (
      await client.query(
        `INSERT INTO chat_session
           (id,workspace_id,agent_id,creator_id,title,status,explicitly_created_at)
         VALUES ($1,$2,$3,$4,$5,'active',now())
         RETURNING id`,
        [randomUUID(), workspace.id, agent.id, owner.id, sessionTitle],
      )
    ).rows[0];
  }
  await client.query("UPDATE chat_session SET title=$1, updated_at=now() WHERE id=$2", [sessionTitle, session.id]);
  await client.query("DELETE FROM chat_issue_link WHERE workspace_id=$1 AND chat_session_id=$2", [workspace.id, session.id]);
  for (const [index, issue] of [...issues.values()].entries()) {
    await client.query(
      `INSERT INTO chat_issue_link (workspace_id,chat_session_id,issue_id,source)
       VALUES ($1,$2,$3,$4)`,
      [workspace.id, session.id, issue.id, index === 0 ? "origin" : "manual"],
    );
  }
  await client.query("COMMIT");
} catch (error) {
  await client.query("ROLLBACK");
  throw error;
}

function mockupLayout(mockup) {
  if (mockup.layout === "calm-mobile") {
    return `
      <rect x="34" y="356" width="352" height="330" rx="28" fill="#111827" stroke="#ffffff" stroke-opacity=".12"/>
      <circle cx="88" cy="430" r="34" fill="${mockup.accent}" fill-opacity=".82"/>
      <path d="M72 430l10 10 20-24" fill="none" stroke="#fff" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>
      <rect x="140" y="395" width="196" height="18" rx="9" fill="#ffffff" fill-opacity=".78"/>
      <rect x="140" y="430" width="172" height="12" rx="6" fill="#ffffff" fill-opacity=".22"/>
      <rect x="66" y="512" width="288" height="68" rx="18" fill="${mockup.accent}" fill-opacity=".82"/>
      <rect x="98" y="616" width="224" height="14" rx="7" fill="#ffffff" fill-opacity=".16"/>`;
  }
  if (mockup.layout === "profile-mobile" || mockup.layout === "profile-mobile-quiet") {
    const actionOpacity = mockup.layout === "profile-mobile-quiet" ? ".24" : ".78";
    return `
      <rect x="34" y="350" width="352" height="368" rx="28" fill="#111827" stroke="#ffffff" stroke-opacity=".12"/>
      <circle cx="98" cy="430" r="42" fill="${mockup.accent}" fill-opacity=".72"/>
      <circle cx="98" cy="418" r="14" fill="#ffffff" fill-opacity=".88"/>
      <path d="M70 456c8-25 48-25 56 0" fill="#ffffff" fill-opacity=".88"/>
      <rect x="164" y="396" width="168" height="18" rx="9" fill="#ffffff" fill-opacity=".82"/>
      <rect x="164" y="432" width="142" height="12" rx="6" fill="#ffffff" fill-opacity=".2"/>
      <rect x="62" y="512" width="86" height="64" rx="16" fill="#ffffff" fill-opacity=".06"/>
      <rect x="166" y="512" width="86" height="64" rx="16" fill="#ffffff" fill-opacity=".06"/>
      <rect x="270" y="512" width="86" height="64" rx="16" fill="#ffffff" fill-opacity=".06"/>
      <rect x="94" y="618" width="232" height="58" rx="18" fill="${mockup.accent}" fill-opacity="${actionOpacity}"/>`;
  }
  if (mockup.layout === "profile-fullpage") {
    return `
      <rect x="90" y="350" width="1020" height="330" rx="36" fill="#111827" stroke="#ffffff" stroke-opacity=".12"/>
      <circle cx="230" cy="500" r="76" fill="${mockup.accent}" fill-opacity=".66"/>
      <rect x="350" y="410" width="390" height="28" rx="14" fill="#ffffff" fill-opacity=".82"/>
      <rect x="350" y="466" width="520" height="16" rx="8" fill="#ffffff" fill-opacity=".2"/>
      <rect x="350" y="536" width="160" height="92" rx="22" fill="#ffffff" fill-opacity=".06"/>
      <rect x="530" y="536" width="160" height="92" rx="22" fill="#ffffff" fill-opacity=".06"/>
      <rect x="710" y="536" width="160" height="92" rx="22" fill="#ffffff" fill-opacity=".06"/>
      <rect x="900" y="438" width="150" height="58" rx="18" fill="${mockup.accent}" fill-opacity=".25"/>
      <rect x="90" y="760" width="650" height="540" rx="36" fill="#ffffff" fill-opacity=".045"/>
      <rect x="780" y="760" width="330" height="540" rx="36" fill="#ffffff" fill-opacity=".045"/>
      <rect x="90" y="1380" width="1020" height="390" rx="36" fill="#ffffff" fill-opacity=".045"/>
      <rect x="90" y="1850" width="1020" height="360" rx="36" fill="#ffffff" fill-opacity=".045"/>`;
  }
  if (mockup.layout === "dense-checkout") {
    return `
      <rect x="170" y="370" width="520" height="300" rx="28" fill="#111827" stroke="#ffffff" stroke-opacity=".12"/>
      <rect x="210" y="414" width="330" height="18" rx="9" fill="#ffffff" fill-opacity=".72"/>
      <rect x="210" y="456" width="420" height="14" rx="7" fill="#ffffff" fill-opacity=".22"/>
      <rect x="210" y="496" width="420" height="58" rx="14" fill="#ffffff" fill-opacity=".06"/>
      <rect x="210" y="574" width="420" height="58" rx="14" fill="#ffffff" fill-opacity=".06"/>
      <rect x="730" y="370" width="520" height="300" rx="28" fill="#111827" stroke="#ffffff" stroke-opacity=".12"/>
      <rect x="770" y="414" width="300" height="18" rx="9" fill="#ffffff" fill-opacity=".72"/>
      <rect x="770" y="456" width="410" height="14" rx="7" fill="#ffffff" fill-opacity=".22"/>
      <rect x="770" y="492" width="410" height="14" rx="7" fill="#ffffff" fill-opacity=".18"/>
      <rect x="770" y="548" width="410" height="70" rx="18" fill="${mockup.accent}" fill-opacity=".72"/>`;
  }
  if (mockup.layout === "calm-checkout") {
    return `
      <rect x="170" y="374" width="1080" height="296" rx="32" fill="#111827" stroke="#ffffff" stroke-opacity=".12"/>
      <circle cx="258" cy="462" r="46" fill="${mockup.accent}" fill-opacity=".82"/>
      <path d="M238 463l14 14 28-32" fill="none" stroke="#fff" stroke-width="10" stroke-linecap="round" stroke-linejoin="round"/>
      <rect x="336" y="414" width="360" height="22" rx="11" fill="#ffffff" fill-opacity=".78"/>
      <rect x="336" y="458" width="510" height="14" rx="7" fill="#ffffff" fill-opacity=".22"/>
      <rect x="336" y="490" width="430" height="14" rx="7" fill="#ffffff" fill-opacity=".16"/>
      <rect x="910" y="426" width="266" height="78" rx="20" fill="${mockup.accent}" fill-opacity=".82"/>
      <rect x="910" y="540" width="266" height="18" rx="9" fill="#ffffff" fill-opacity=".18"/>
      <rect x="910" y="580" width="190" height="14" rx="7" fill="#ffffff" fill-opacity=".12"/>`;
  }
  return `
    <rect x="170" y="362" width="1080" height="316" rx="34" fill="#111827" stroke="#ffffff" stroke-opacity=".12"/>
    <circle cx="278" cy="470" r="66" fill="${mockup.accent}" fill-opacity=".72"/>
    <circle cx="278" cy="448" r="22" fill="#ffffff" fill-opacity=".88"/>
    <path d="M232 514c12-38 80-38 92 0" fill="#ffffff" fill-opacity=".88"/>
    <rect x="384" y="410" width="320" height="24" rx="12" fill="#ffffff" fill-opacity=".82"/>
    <rect x="384" y="458" width="430" height="14" rx="7" fill="#ffffff" fill-opacity=".2"/>
    <rect x="384" y="530" width="148" height="82" rx="20" fill="#ffffff" fill-opacity=".06"/>
    <rect x="550" y="530" width="148" height="82" rx="20" fill="#ffffff" fill-opacity=".06"/>
    <rect x="716" y="530" width="148" height="82" rx="20" fill="#ffffff" fill-opacity=".06"/>
    <rect x="936" y="428" width="240" height="76" rx="20" fill="${mockup.accent}" fill-opacity=".8"/>`;
}

function renderMockup(mockup) {
  const width = mockup.width ?? 1440;
  const height = mockup.height ?? 900;
  const shellX = width <= 500 ? 20 : 96;
  const shellY = width <= 500 ? 52 : 82;
  const shellWidth = width - shellX * 2;
  const shellHeight = height - shellY * 2;
  const textX = width <= 500 ? 34 : width <= 1250 ? 90 : 170;
  const titleSize = width <= 500 ? 38 : 62;
  const subtitleSize = width <= 500 ? 18 : 27;
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">
    <defs>
      <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#171827"/><stop offset="1" stop-color="#090d14"/></linearGradient>
      <radialGradient id="glow"><stop offset="0" stop-color="${mockup.accent}" stop-opacity=".48"/><stop offset="1" stop-color="${mockup.accent}" stop-opacity="0"/></radialGradient>
      <filter id="blur"><feGaussianBlur stdDeviation="62"/></filter>
    </defs>
    <rect width="${width}" height="${height}" fill="url(#bg)"/>
    <circle cx="${Math.round(width * .8)}" cy="${Math.round(height * .16)}" r="${Math.round(Math.min(width, height) * .36)}" fill="url(#glow)" filter="url(#blur)"/>
    <rect x="${shellX}" y="${shellY}" width="${shellWidth}" height="${shellHeight}" rx="44" fill="#ffffff" fill-opacity=".045" stroke="#ffffff" stroke-opacity=".13"/>
    <text x="${textX}" y="164" fill="${mockup.accent}" font-family="system-ui,sans-serif" font-size="18" font-weight="700" letter-spacing="1.8">${mockup.eyebrow}</text>
    <text x="${textX}" y="254" fill="#ffffff" font-family="system-ui,sans-serif" font-size="${titleSize}" font-weight="720">${mockup.title}</text>
    <text x="${textX}" y="316" fill="#b8beca" font-family="system-ui,sans-serif" font-size="${subtitleSize}">${mockup.subtitle}</text>
    ${mockupLayout(mockup)}
    <text x="${textX}" y="${height - 58}" fill="#8e96a5" font-family="system-ui,sans-serif" font-size="18">Local synthetic fixture · no agent or provider ran</text>
  </svg>`;
}

function renderPrototypePage(artifact) {
  return `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${artifact.label}</title><style>body{margin:0;background:#0b0d14;color:#f8fafc;font:16px system-ui}main{max-width:1100px;margin:auto;padding:64px 32px}.hero,.panel{border:1px solid #ffffff20;background:#ffffff08;border-radius:28px}.hero{padding:56px;background:radial-gradient(circle at 80% 0,${artifact.accent}66,transparent 42%),#151827}.eyebrow{color:${artifact.accent};font-weight:700;letter-spacing:.14em}.grid{display:grid;grid-template-columns:2fr 1fr;gap:24px;margin-top:24px}.panel{min-height:320px;padding:32px}.line{height:14px;border-radius:7px;background:#ffffff25;margin:18px 0}.cta{height:64px;border-radius:18px;background:${artifact.accent};margin-top:40px}@media(max-width:700px){.grid{grid-template-columns:1fr}}</style><main><section class="hero"><div class="eyebrow">FULL PAGE PROTOTYPE</div><h1>Checkout onboarding</h1><p>One complete, scrollable page delivered with the supporting screenshots and handoff.</p></section><div class="grid"><section class="panel"><h2>Review the account</h2><div class="line"></div><div class="line"></div><div class="line"></div><div class="cta"></div></section><aside class="panel"><h2>What happens next</h2><div class="line"></div><div class="line"></div></aside></div><section class="panel" style="margin-top:24px"><h2>Supporting details</h2><div class="line"></div><div class="line"></div><div class="line"></div></section></main>`;
}

function renderMinimalPdf(label) {
  const escaped = label.replace(/[()\\]/g, "\\$&");
  const stream = `BT /F1 22 Tf 72 700 Td (${escaped}) Tj 0 -36 Td /F1 12 Tf (Synthetic local handoff - no provider ran.) Tj ET`;
  const objects = [
    "<< /Type /Catalog /Pages 2 0 R >>",
    "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
    "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    `<< /Length ${Buffer.byteLength(stream)} >>\nstream\n${stream}\nendstream`,
  ];
  let body = "%PDF-1.4\n";
  const offsets = [0];
  objects.forEach((object, index) => { offsets.push(Buffer.byteLength(body)); body += `${index + 1} 0 obj\n${object}\nendobj\n`; });
  const xref = Buffer.byteLength(body);
  body += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n${offsets.slice(1).map((offset) => `${String(offset).padStart(10, "0")} 00000 n `).join("\n")}\ntrailer << /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF`;
  return Buffer.from(body);
}

async function ensureArtifactAttachment(artifact) {
  const issue = issues.get(artifact.issueKey);
  const existing = (
    await client.query(
      `SELECT id FROM attachment
       WHERE workspace_id=$1 AND issue_id=$2 AND filename=$3
       ORDER BY created_at DESC LIMIT 1`,
      [workspace.id, issue.id, artifact.filename],
    )
  ).rows[0];
  if (existing) return existing.id;

  const content = artifact.kind === "image"
    ? await sharp(Buffer.from(renderMockup(artifact))).png().toBuffer()
    : artifact.kind === "page"
      ? Buffer.from(renderPrototypePage(artifact))
      : renderMinimalPdf(artifact.label);
  const mime = artifact.kind === "image" ? "image/png" : artifact.kind === "page" ? "text/html" : "application/pdf";
  const form = new FormData();
  form.append("file", new Blob([content], { type: mime }), artifact.filename);
  form.append("issue_id", issue.id);
  const response = await fetch(apiBase + "/api/upload-file", {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "X-Workspace-ID": workspace.id },
    body: form,
  });
  if (!response.ok) throw new Error(`Local conversation mockup upload: HTTP ${response.status}`);
  return (await response.json()).id;
}

const attachments = new Map();
for (const artifact of scenario.artifacts) attachments.set(artifact.key, await ensureArtifactAttachment(artifact));

let transcriptCommitted = false;
await client.query("BEGIN");
try {
  const issueIds = [...issues.values()].map((issue) => issue.id);
  const fixtureMessageIds = scenario.timeline.map((entry) => stableFixtureUUID(`message:${entry.key}`));
  const fixtureContents = scenario.timeline.filter((entry) => entry.content).map((entry) => entry.content);
  await client.query(
    `DELETE FROM chat_message
     WHERE chat_session_id=$1 AND (
       id = ANY($2::uuid[])
       OR (message_kind='work_update' AND work_issue_id = ANY($3::uuid[]))
       OR content = ANY($4::text[])
     )`,
    [session.id, fixtureMessageIds, issueIds, fixtureContents],
  );
  await client.query(
    `DELETE FROM inbox_item
     WHERE workspace_id=$1 AND details->>'fixture' = ANY($2::text[])`,
    [workspace.id, ["owner-review-acceptance", "owner-review-conversation"]],
  );

  const baseTime = Date.now();
  const requestIds = new Map();
  for (const entry of scenario.timeline.filter((item) => item.kind === "question")) {
    const issue = issues.get(entry.issueKey);
    const requestId = stableFixtureUUID(`request:${entry.issueKey}`);
    requestIds.set(entry.issueKey, requestId);
    const createdAt = new Date(baseTime + entry.at * 60_000).toISOString();
    await client.query(
      `INSERT INTO inbox_item
         (id,workspace_id,recipient_type,recipient_id,type,severity,issue_id,title,body,
          actor_type,details,created_at)
       VALUES ($1,$2,'member',$3,'workflow_owner_decision','action_required',$4,$5,$6,
          'system',$7,$8)`,
      [
        requestId,
        workspace.id,
        login.user.id,
        issue.id,
        issue.title,
        issue.question.body,
        {
          owner_relevant: "true",
          owner_category: "needs_you",
          required_action: "answer_design_direction",
          question: issue.question.body,
          options: issue.question.options.join("\n"),
          recommendation: issue.question.recommendation,
          consequence: issue.question.consequence,
          fixture: "owner-review-conversation",
          issue_key: issue.key,
        },
        createdAt,
      ],
    );
  }

  for (const entry of scenario.timeline) {
    const createdAt = new Date(baseTime + entry.at * 60_000).toISOString();
    if (entry.kind === "message") {
      await client.query(
        `INSERT INTO chat_message (id,chat_session_id,role,content,message_kind,created_at)
         VALUES ($1,$2,$3,$4,'message',$5)`,
        [stableFixtureUUID(`message:${entry.key}`), session.id, entry.role, entry.content, createdAt],
      );
      continue;
    }

    const issue = issues.get(entry.issueKey);
    // Owner questions are delivered by the real Chat work-delivery path from
    // the canonical Inbox request inserted above. Inserting a second synthetic
    // question message here would duplicate the request in the conversation.
    if (entry.kind === "question") continue;
    if (entry.kind === "result") {
      const deliveredArtifacts = entry.artifactKeys.map((key) => {
        const artifact = scenario.artifacts.find((item) => item.key === key);
        const attachmentId = attachments.get(key);
        return {
          id: attachmentId,
          label: artifact.label,
          kind: artifact.kind,
          url: artifact.kind === "page"
            ? `/${workspaceSlug}/attachments/${attachmentId}/preview`
            : `/api/attachments/${attachmentId}/download`,
          original: true,
        };
      });
      const workUpdate = {
        kind: "result",
        issue_id: issue.id,
        title: issue.title,
        source_id: stableFixtureUUID(`source:${entry.key}`),
        occurred_at: createdAt,
        body: entry.content,
        review: {
          schema_version: 1,
          source_task_id: stableFixtureUUID(`task:${entry.key}`),
          artifacts: deliveredArtifacts,
        },
      };
      await client.query(
        `INSERT INTO chat_message
           (id,chat_session_id,role,content,message_kind,work_issue_id,work_update,
            work_seen_at,work_follow_up_eligible,created_at)
         VALUES ($1,$2,'assistant',$3,'work_update',$4,$5,now(),false,$6)`,
        [stableFixtureUUID(`message:${entry.key}`), session.id, workUpdate.body, issue.id, workUpdate, createdAt],
      );
    }
  }
  await client.query("UPDATE chat_session SET updated_at=now() WHERE id=$1", [session.id]);
  await client.query("COMMIT");
  transcriptCommitted = true;

  const expectedRequestIds = [...requestIds.values()];
  let deliveredQuestions = 0;
  for (let attempt = 0; attempt < 25; attempt++) {
    deliveredQuestions = Number((
      await client.query(
        `SELECT count(*) FROM chat_message
         WHERE chat_session_id=$1 AND message_kind='work_update'
           AND work_update->>'kind'='question'
           AND work_update->>'request_id' = ANY($2::text[])`,
        [session.id, expectedRequestIds],
      )
    ).rows[0].count);
    if (deliveredQuestions === expectedRequestIds.length) break;
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
  if (deliveredQuestions !== expectedRequestIds.length) {
    throw new Error(`Expected ${expectedRequestIds.length} canonical Chat questions, found ${deliveredQuestions}.`);
  }
} catch (error) {
  if (!transcriptCommitted) await client.query("ROLLBACK");
  throw error;
} finally {
  token = "";
  await client.end();
}

console.log(
  JSON.stringify(
    {
      boundary: "Synthetic local multi-issue conversation; no task, runtime, model, provider, or production resource.",
      workspace_id: workspace.id,
      issues: [...issues.values()].map((issue) => ({ id: issue.id, number: issue.number, title: issue.title })),
      chat_session_id: session.id,
      attachment_ids: Object.fromEntries(attachments),
      chat_url: `${webBase}/${workspaceSlug}/chat?session=${session.id}`,
      inbox_url: `${webBase}/${workspaceSlug}/inbox`,
    },
    null,
    2,
  ),
);
